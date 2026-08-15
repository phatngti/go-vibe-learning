// File: internal/platform/kafka/consumer.go
// Description: Kafka subscriber/consumer logic with error retry loop and DLQ publishing.

package kafka

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/observability/logger"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
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
	brokers                   []string
	producer                  Producer
	logger                    logger.Factory
	mu                        sync.Mutex
	readers                   []*kafkago.Reader
	maxAttempts               int
	retryMaxDelay             time.Duration
	fetchMaxBytes             int
	sessionTimeout            time.Duration
	heartbeatInterval         time.Duration
	deadLetterMaxPayloadBytes int
	dialer                    *kafkago.Dialer
	codec                     MessageCodec
}

func NewConsumer(cfg *platformconfig.Kafka, producer Producer, l logger.Factory) (Consumer, error) {
	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker is required")
	}
	tlsConfig, saslMechanism, err := kafkaSecurity(cfg)
	if err != nil {
		return nil, err
	}
	maxAttempts := cfg.ConsumerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	retryMaxDelay := cfg.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = 30 * time.Second
	}
	fetchMaxBytes := cfg.FetchMaxBytes
	if fetchMaxBytes <= 0 {
		fetchMaxBytes = 10 << 20
	}
	sessionTimeout := cfg.SessionTimeout
	if sessionTimeout <= 0 {
		sessionTimeout = 30 * time.Second
	}
	heartbeatInterval := cfg.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 3 * time.Second
	}
	if heartbeatInterval >= sessionTimeout {
		return nil, fmt.Errorf("kafka: heartbeat interval %s must be shorter than session timeout %s", heartbeatInterval, sessionTimeout)
	}
	dlqMaxPayload := cfg.DeadLetterMaxPayloadBytes
	if dlqMaxPayload <= 0 {
		dlqMaxPayload = 256 << 10
	}
	codec, err := NewMessageCodec(cfg)
	if err != nil {
		return nil, err
	}
	return &consumer{
		brokers: cfg.Brokers, producer: producer, logger: l,
		maxAttempts: maxAttempts, retryMaxDelay: retryMaxDelay,
		fetchMaxBytes: fetchMaxBytes, sessionTimeout: sessionTimeout,
		heartbeatInterval: heartbeatInterval, deadLetterMaxPayloadBytes: dlqMaxPayload,
		dialer: &kafkago.Dialer{Timeout: 10 * time.Second, DualStack: true, TLS: tlsConfig, SASLMechanism: saslMechanism},
		codec:  codec,
	}, nil
}

func (c *consumer) Subscribe(ctx context.Context, topic, groupID string, handler Handler) error {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: c.brokers, Topic: topic, GroupID: groupID,
		Dialer: c.dialer, GroupBalancers: []kafkago.GroupBalancer{kafkago.RoundRobinGroupBalancer{}},
		MinBytes: 1, MaxBytes: c.fetchMaxBytes, CommitInterval: 0,
		StartOffset: kafkago.FirstOffset, MaxWait: 500 * time.Millisecond,
		SessionTimeout: c.sessionTimeout, HeartbeatInterval: c.heartbeatInterval,
		IsolationLevel: kafkago.ReadCommitted, MaxAttempts: c.maxAttempts,
	})
	c.registerReader(reader)
	defer func() {
		c.unregisterReader(reader)
		_ = reader.Close()
	}()

	for {
		raw, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("kafka: fetch %q: %w", topic, err)
		}

		msg, err := c.codec.Decode(ctx, raw.Topic, raw.Value)
		if err != nil {
			if err := c.deadLetterWithRetry(ctx, groupID, raw, "", "decode", err); err != nil {
				return err
			}
			if err := c.commitWithRetry(ctx, reader, topic, raw); err != nil {
				return err
			}
			kafkaConsumed.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("outcome", "dead_letter")))
			continue
		}
		normalizeMessageAliases(msg)
		if err := validateMessage(msg); err != nil {
			if err := c.deadLetterWithRetry(ctx, groupID, raw, msg.ID, "validation", err); err != nil {
				return err
			}
			if err := c.commitWithRetry(ctx, reader, topic, raw); err != nil {
				return err
			}
			kafkaConsumed.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("outcome", "dead_letter")))
			continue
		}

		attempt := 0
		deadLettered := false
		handlerCtx := extractTraceContext(ctx, raw.Headers, msg)
		for {
			startedAt := time.Now()
			handlerErr := handler(handlerCtx, msg)
			kafkaHandlerDuration.Record(ctx, time.Since(startedAt).Seconds(), metric.WithAttributes(
				attribute.String("topic", topic), attribute.String("event_type", msg.EventType)))
			if handlerErr == nil {
				break
			}
			if IsPermanent(handlerErr) {
				if err := c.deadLetterWithRetry(ctx, groupID, raw, msg.ID, "permanent", handlerErr); err != nil {
					return err
				}
				deadLettered = true
				break
			}
			attempt++
			if attempt >= c.maxAttempts {
				if err := c.deadLetterWithRetry(ctx, groupID, raw, msg.ID, "retry_exhausted", fmt.Errorf("retry budget exhausted after %d attempts: %w", attempt, handlerErr)); err != nil {
					return err
				}
				deadLettered = true
				break
			}
			kafkaConsumerRetries.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("event_type", msg.EventType)))
			delay := min(time.Duration(1<<min(attempt-1, 8))*time.Second, c.retryMaxDelay)
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

		if err := c.commitWithRetry(ctx, reader, topic, raw); err != nil {
			return err
		}
		outcome := "handled"
		if deadLettered {
			outcome = "dead_letter"
		}
		kafkaConsumed.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("outcome", outcome)))
	}
}

func extractTraceContext(ctx context.Context, headers []kafkago.Header, msg *Message) context.Context {
	carrier := propagation.MapCarrier{}
	for _, header := range headers {
		carrier.Set(header.Key, string(header.Value))
	}
	if msg != nil {
		if carrier.Get("traceparent") == "" && msg.TraceParent != "" {
			carrier.Set("traceparent", msg.TraceParent)
		}
		if carrier.Get("tracestate") == "" && msg.TraceState != "" {
			carrier.Set("tracestate", msg.TraceState)
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func validateMessage(msg *Message) error {
	if msg.ID == "" {
		return errors.New("kafka: message id is required")
	}
	if msg.EventType == "" {
		return errors.New("kafka: event type is required")
	}
	if len(msg.Payload) == 0 && msg.ProtobufPayload == nil {
		return errors.New("kafka: payload is required")
	}
	if msg.AggregateVersion < 0 {
		return errors.New("kafka: aggregate version cannot be negative")
	}
	if (msg.OrchestrationOwner == "") != (msg.OrchestrationGeneration == 0) {
		return errors.New("kafka: orchestration owner and generation must be provided together")
	}
	if msg.OrchestrationGeneration < 0 {
		return errors.New("kafka: orchestration generation cannot be negative")
	}
	return nil
}

func (c *consumer) deadLetterWithRetry(ctx context.Context, groupID string, raw kafkago.Message, sourceMessageID, reasonClass string, cause error) error {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := c.deadLetter(ctx, groupID, raw, sourceMessageID, reasonClass, cause); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if err := waitForRetry(ctx, min(time.Duration(1<<min(attempt-1, 8))*time.Second, c.retryMaxDelay)); err != nil {
			return nil
		}
	}
	return lastErr
}

func (c *consumer) deadLetter(ctx context.Context, groupID string, raw kafkago.Message, sourceMessageID, reasonClass string, cause error) error {
	digest := sha256.Sum256(raw.Value)
	rawPayload := raw.Value
	truncated := len(rawPayload) > c.deadLetterMaxPayloadBytes
	if truncated {
		rawPayload = rawPayload[:c.deadLetterMaxPayloadBytes]
	}
	payload, err := json.Marshal(messagebus.DeadLetter{
		SourceTopic: raw.Topic, SourceConsumer: groupID, SourcePartition: raw.Partition,
		SourceOffset: raw.Offset, SourceMessageID: sourceMessageID, SourceKey: raw.Key,
		ReasonClass: reasonClass, Reason: truncateConsumerError(cause.Error(), 4096),
		RawMessage: rawPayload, RawMessageSHA256: fmt.Sprintf("%x", digest[:]), RawMessageTruncated: truncated,
	})
	if err != nil {
		return fmt.Errorf("kafka: marshal dead letter for %q: %w", raw.Topic, err)
	}
	dlqID := fmt.Sprintf("dlq:%s:%s:%d:%d", groupID, raw.Topic, raw.Partition, raw.Offset)
	msg := &Message{
		ID:        dlqID,
		EventType: messagebus.DeadLetterType, AggregateType: "DeadLetter",
		AggregateID: dlqID, Payload: payload, OccurredAt: time.Now(),
	}
	if err := c.producer.Publish(ctx, raw.Topic+".dlq", dlqID, msg); err != nil {
		return fmt.Errorf("kafka: publish dead letter for %q: %w", raw.Topic, err)
	}
	kafkaDeadLetters.Add(ctx, 1, metric.WithAttributes(attribute.String("source_topic", raw.Topic), attribute.String("reason_class", reasonClass)))
	return nil
}

func (c *consumer) commitWithRetry(ctx context.Context, reader *kafkago.Reader, topic string, raw kafkago.Message) error {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := reader.CommitMessages(ctx, raw); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if err := waitForRetry(ctx, min(time.Duration(1<<min(attempt-1, 8))*time.Second, c.retryMaxDelay)); err != nil {
			return nil
		}
	}
	return fmt.Errorf("kafka: commit %q after %d attempts: %w", topic, c.maxAttempts, lastErr)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func truncateConsumerError(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func (c *consumer) registerReader(reader *kafkago.Reader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readers = append(c.readers, reader)
}

func (c *consumer) unregisterReader(reader *kafkago.Reader) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for index, candidate := range c.readers {
		if candidate == reader {
			c.readers = append(c.readers[:index], c.readers[index+1:]...)
			return
		}
	}
}

func (c *consumer) Close() error {
	c.mu.Lock()
	readers := append([]*kafkago.Reader(nil), c.readers...)
	c.mu.Unlock()
	var lastErr error
	for _, reader := range readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
