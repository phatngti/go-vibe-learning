// File: internal/platform/kafka/message.go
// Description: Defines the Message struct representing the Kafka topic event envelope.

package kafka

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	kafkaMeter              = otel.Meter("github/master_go_cqrs/internal/platform/kafka")
	kafkaProduced, _        = kafkaMeter.Int64Counter("kafka.producer.messages")
	kafkaProduceDuration, _ = kafkaMeter.Float64Histogram("kafka.producer.duration", metric.WithUnit("s"))
	kafkaConsumed, _        = kafkaMeter.Int64Counter("kafka.consumer.messages")
	kafkaConsumerRetries, _ = kafkaMeter.Int64Counter("kafka.consumer.retries")
	kafkaDeadLetters, _     = kafkaMeter.Int64Counter("kafka.consumer.dead_letters")
	kafkaHandlerDuration, _ = kafkaMeter.Float64Histogram("kafka.consumer.handler.duration", metric.WithUnit("s"))
)

// Message is the typed envelope written to and read from Kafka topics. Legacy
// topics retain JSON in Payload; v2 Protobuf topics carry a typed Any payload
// and use Schema Registry framing without falling back to JSON on failure.
type Message struct {
	// SchemaVersion enables backward-compatible contract evolution.
	SchemaVersion int `json:"schema_version"`
	// ID is the legacy event/message identifier retained for v1 consumers.
	ID string `json:"id"`
	// EventID is the v2 alias of ID. Producers populate both fields during the
	// rolling JSON-to-Protobuf migration so old and new consumers can coexist.
	EventID string `json:"event_id,omitempty"`
	// EventType is the name of the domain event, e.g. "PaymentCompleted".
	EventType string `json:"event_type"`
	// AggregateType is the domain entity type, e.g. "Transaction".
	AggregateType string `json:"aggregate_type"`
	// AggregateID is the entity identifier.
	AggregateID string `json:"aggregate_id"`
	// Payload holds the JSON-encoded domain event struct.
	Payload []byte `json:"payload"`
	// ProtobufPayload carries a typed v2 payload. During migration, producers
	// that only provide Payload are wrapped in google.protobuf.BytesValue so
	// existing JSON handlers can still round-trip through a dedicated v2 topic.
	ProtobufPayload *anypb.Any `json:"-"`
	// Version is the legacy aggregate-version field retained for v1 consumers.
	Version int64 `json:"version,omitempty"`
	// AggregateVersion is the explicit v2 aggregate-version field.
	AggregateVersion int64 `json:"aggregate_version,omitempty"`
	// OccurredAt is when the event happened (set by the producer).
	OccurredAt              time.Time `json:"occurred_at"`
	TenantID                string    `json:"tenant_id,omitempty"`
	CorrelationID           string    `json:"correlation_id,omitempty"`
	CausationID             string    `json:"causation_id,omitempty"`
	OrchestrationOwner      string    `json:"orchestration_owner,omitempty"`
	OrchestrationGeneration int64     `json:"orchestration_generation,omitempty"`
	TraceParent             string    `json:"traceparent,omitempty"`
	TraceState              string    `json:"tracestate,omitempty"`
}

// PrepareMessage fills compatibility aliases and durable envelope metadata
// before a message is serialized or stored in an Outbox. Existing v1 fields
// remain populated while v2 consumers migrate to the explicit names.
func PrepareMessage(ctx context.Context, msg *Message) propagation.MapCarrier {
	if msg == nil {
		return propagation.MapCarrier{}
	}
	normalizeMessageAliases(msg)

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if carrier.Get("traceparent") == "" && msg.TraceParent != "" {
		carrier.Set("traceparent", msg.TraceParent)
	}
	if carrier.Get("tracestate") == "" && msg.TraceState != "" {
		carrier.Set("tracestate", msg.TraceState)
	}
	if msg.TraceParent == "" {
		msg.TraceParent = carrier.Get("traceparent")
	}
	if msg.TraceState == "" {
		msg.TraceState = carrier.Get("tracestate")
	}
	return carrier
}

func normalizeMessageAliases(msg *Message) {
	if msg.ID == "" {
		msg.ID = msg.EventID
	}
	if msg.EventID == "" {
		msg.EventID = msg.ID
	}
	if msg.AggregateVersion == 0 {
		msg.AggregateVersion = msg.Version
	}
	if msg.Version == 0 {
		msg.Version = msg.AggregateVersion
	}
	if msg.SchemaVersion == 0 {
		msg.SchemaVersion = 1
	}
	if msg.CorrelationID == "" {
		msg.CorrelationID = msg.AggregateID
	}
}
