// File: internal/platform/kafka/producer.go
// Description: Kafka message publisher/producer implementation wrapper.

package kafka

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Producer interface {
	Publish(ctx context.Context, topic, key string, msg *Message) error
	Close() error
}

type producer struct {
	brokers     []string
	transport   *kafkago.Transport
	settings    platformconfig.KafkaProducer
	compression kafkago.Compression
	codec       MessageCodec
	mu          sync.Mutex
	writers     map[string]*kafkago.Writer
}

func NewProducer(cfg *platformconfig.Kafka) (Producer, error) {
	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker is required")
	}
	tlsConfig, saslMechanism, err := kafkaSecurity(cfg)
	if err != nil {
		return nil, err
	}
	settings := cfg.Producer
	if settings.MaxAttempts <= 0 {
		settings.MaxAttempts = 10
	}
	if settings.BatchSize <= 0 {
		settings.BatchSize = 100
	}
	if settings.BatchBytes <= 0 {
		settings.BatchBytes = 1 << 20
	}
	if settings.BatchTimeout <= 0 {
		settings.BatchTimeout = 10 * time.Millisecond
	}
	if settings.ReadTimeout <= 0 {
		settings.ReadTimeout = 10 * time.Second
	}
	if settings.WriteTimeout <= 0 {
		settings.WriteTimeout = 10 * time.Second
	}
	if settings.DeliveryTimeout <= 0 {
		settings.DeliveryTimeout = 30 * time.Second
	}
	compression, err := kafkaCompression(settings.Compression)
	if err != nil {
		return nil, err
	}
	codec, err := NewMessageCodec(cfg)
	if err != nil {
		return nil, err
	}
	return &producer{
		brokers: cfg.Brokers, writers: make(map[string]*kafkago.Writer),
		transport: &kafkago.Transport{TLS: tlsConfig, SASL: saslMechanism},
		settings:  settings, compression: compression, codec: codec,
	}, nil
}

func (p *producer) writerFor(topic string) *kafkago.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()
	if writer, ok := p.writers[topic]; ok {
		return writer
	}
	writer := &kafkago.Writer{
		Addr: kafkago.TCP(p.brokers...), Topic: topic,
		Transport: p.transport, Balancer: &kafkago.Hash{},
		MaxAttempts: p.settings.MaxAttempts, BatchSize: p.settings.BatchSize,
		BatchBytes: p.settings.BatchBytes, BatchTimeout: p.settings.BatchTimeout,
		ReadTimeout: p.settings.ReadTimeout, WriteTimeout: p.settings.WriteTimeout,
		Compression: p.compression, AllowAutoTopicCreation: false,
		RequiredAcks: kafkago.RequireAll, Async: false,
	}
	p.writers[topic] = writer
	return writer
}

func (p *producer) Publish(ctx context.Context, topic, key string, msg *Message) error {
	if msg.ID == "" && msg.EventID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.OccurredAt.IsZero() {
		msg.OccurredAt = time.Now()
	}
	carrier := PrepareMessage(ctx, msg)
	data, err := p.codec.Encode(ctx, topic, msg)
	if err != nil {
		return fmt.Errorf("kafka: encode message: %w", err)
	}
	if key == "" {
		key = msg.AggregateID
	}
	headers := make([]kafkago.Header, 0, len(carrier))
	for name, value := range carrier {
		headers = append(headers, kafkago.Header{Key: name, Value: []byte(value)})
	}
	startedAt := time.Now()
	topicAttr := metric.WithAttributes(attribute.String("topic", topic))
	publishCtx, cancel := context.WithTimeout(ctx, p.settings.DeliveryTimeout)
	defer cancel()
	if err := p.writerFor(topic).WriteMessages(publishCtx, kafkago.Message{Key: []byte(key), Value: data, Headers: headers}); err != nil {
		kafkaProduceDuration.Record(ctx, time.Since(startedAt).Seconds(), topicAttr)
		kafkaProduced.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("outcome", "error")))
		return fmt.Errorf("kafka: publish to %q: %w", topic, err)
	}
	kafkaProduceDuration.Record(ctx, time.Since(startedAt).Seconds(), topicAttr)
	kafkaProduced.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic), attribute.String("outcome", "success")))
	return nil
}

func kafkaCompression(value string) (kafkago.Compression, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "lz4":
		return kafkago.Lz4, nil
	case "none", "uncompressed":
		return 0, nil
	case "gzip":
		return kafkago.Gzip, nil
	case "snappy":
		return kafkago.Snappy, nil
	case "zstd":
		return kafkago.Zstd, nil
	default:
		return 0, fmt.Errorf("kafka: unsupported producer compression %q", value)
	}
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
	p.transport.CloseIdleConnections()
	return lastErr
}
