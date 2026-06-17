// File: internal/platform/kafka/message.go
// Description: Defines the Message struct representing the Kafka topic event envelope.

package kafka

import (
	"time"
)

// Message is the typed envelope written to and read from Kafka topics.
// All domain events are serialized as JSON inside Payload.
type Message struct {
	// ID is a deterministic identifier for the transaction step.
	ID string `json:"id"`
	// EventType is the name of the domain event, e.g. "PaymentCompleted".
	EventType string `json:"event_type"`
	// AggregateType is the domain entity type, e.g. "Transaction".
	AggregateType string `json:"aggregate_type"`
	// AggregateID is the entity identifier.
	AggregateID string `json:"aggregate_id"`
	// Payload holds the JSON-encoded domain event struct.
	Payload []byte `json:"payload"`
	// Version is the aggregate version this event corresponds to.
	Version int64 `json:"version"`
	// OccurredAt is when the event happened (set by the producer).
	OccurredAt time.Time `json:"occurred_at"`
}
