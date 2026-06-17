// File: internal/platform/kafka/producer.go
// Description: Kafka message publisher/producer implementation wrapper.

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

type Producer interface {
	Publish(ctx context.Context, topic, key string, msg *Message) error
	Close() error
}

type producer struct {
	brokers []string
	mu      sync.Mutex
	writers map[string]*kafkago.Writer
}

func NewProducer(brokers []string) Producer {
	return &producer{brokers: brokers, writers: make(map[string]*kafkago.Writer)}
}

func (p *producer) writerFor(topic string) *kafkago.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if writer, ok := p.writers[topic]; ok {
		return writer
	}
	writer := &kafkago.Writer{
		Addr: kafkago.TCP(p.brokers...), Topic: topic,
		Balancer: &kafkago.Hash{}, BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafkago.RequireOne, Async: false,
	}
	p.writers[topic] = writer
	return writer
}

func (p *producer) Publish(ctx context.Context, topic, key string, msg *Message) error {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.OccurredAt.IsZero() {
		msg.OccurredAt = time.Now()
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("kafka: marshal message: %w", err)
	}
	if key == "" {
		key = msg.AggregateID
	}
	if err := p.writerFor(topic).WriteMessages(ctx, kafkago.Message{Key: []byte(key), Value: data}); err != nil {
		return fmt.Errorf("kafka: publish to %q: %w", topic, err)
	}
	return nil
}

func (p *producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var lastErr error
	for _, writer := range p.writers {
		if err := writer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
