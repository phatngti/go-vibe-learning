// File: internal/platform/kafka/consumer.go
// Description: Kafka subscriber/consumer logic with error retry loop and DLQ publishing.

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/observability/logger"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Handler func(ctx context.Context, msg *Message) error

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

func IsPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
}

type Consumer interface {
	Subscribe(ctx context.Context, topic, groupID string, handler Handler) error
	Close() error
}

type consumer struct {
	brokers  []string
	producer Producer
	logger   logger.Factory
	mu       sync.Mutex
	readers  []*kafkago.Reader
}

func NewConsumer(brokers []string, producer Producer, l logger.Factory) Consumer {
	return &consumer{brokers: brokers, producer: producer, logger: l}
}

func (c *consumer) Subscribe(ctx context.Context, topic, groupID string, handler Handler) error {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: c.brokers, Topic: topic, GroupID: groupID,
		MinBytes: 1, MaxBytes: 10e6, CommitInterval: 0,
		StartOffset: kafkago.FirstOffset, MaxWait: 500 * time.Millisecond,
	})
	c.mu.Lock()
	c.readers = append(c.readers, reader)
	c.mu.Unlock()
	defer reader.Close()

	for {
		raw, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("kafka: fetch %q: %w", topic, err)
		}

		var msg Message
		if err := json.Unmarshal(raw.Value, &msg); err != nil {
			if err := c.deadLetter(ctx, topic, raw.Key, raw.Value, err); err != nil {
				return err
			}
			if err := reader.CommitMessages(ctx, raw); err != nil {
				return err
			}
			continue
		}
		if err := validateMessage(&msg); err != nil {
			if err := c.deadLetter(ctx, topic, raw.Key, raw.Value, err); err != nil {
				return err
			}
			if err := reader.CommitMessages(ctx, raw); err != nil {
				return err
			}
			continue
		}

		attempt := 0
		for {
			handlerErr := handler(ctx, &msg)
			if handlerErr == nil {
				break
			}
			if IsPermanent(handlerErr) {
				encoded, _ := json.Marshal(msg)
				if err := c.deadLetter(ctx, topic, raw.Key, encoded, handlerErr); err != nil {
					return err
				}
				break
			}
			attempt++
			delay := time.Duration(1<<min(attempt-1, 5)) * time.Second
			c.logger.Bg().Warn("Kafka handler failed; retrying",
				zap.String("topic", topic), zap.String("message_id", msg.ID),
				zap.Duration("delay", delay), zap.Error(handlerErr))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}

		if err := reader.CommitMessages(ctx, raw); err != nil {
			return fmt.Errorf("kafka: commit %q: %w", topic, err)
		}
	}
}

func validateMessage(msg *Message) error {
	if msg.ID == "" {
		return errors.New("kafka: message id is required")
	}
	if msg.EventType == "" {
		return errors.New("kafka: event type is required")
	}
	if len(msg.Payload) == 0 {
		return errors.New("kafka: payload is required")
	}
	return nil
}

func (c *consumer) deadLetter(ctx context.Context, topic string, key, raw []byte, cause error) error {
	payload, _ := json.Marshal(messagebus.DeadLetter{SourceTopic: topic, Reason: cause.Error(), RawMessage: raw})
	msg := &Message{
		ID:        fmt.Sprintf("dlq:%s:%d", topic, time.Now().UnixNano()),
		EventType: messagebus.DeadLetterType, AggregateType: "DeadLetter",
		AggregateID: string(key), Payload: payload,
	}
	if err := c.producer.Publish(ctx, topic+".dlq", string(key), msg); err != nil {
		return fmt.Errorf("kafka: publish dead letter for %q: %w", topic, err)
	}
	return nil
}

func (c *consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var lastErr error
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
