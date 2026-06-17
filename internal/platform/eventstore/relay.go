// File: internal/platform/eventstore/relay.go
// Description: Relay wrapper that automatically copies appended domain events into the transactional outbox.

package eventstore

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/platform/outbox"

	"gorm.io/gorm"
)

// RelayMapping describes how a single domain event type should be forwarded to the outbox.
type RelayMapping struct {
	// Topic is the Kafka topic to publish to.
	Topic string
	// MessageType is the outbox message type, e.g. messagebus.PaymentCompletedType.
	MessageType string
	// StepName is used to generate a deterministic MessageID: "{AggregateID}:{StepName}".
	StepName string
}

// RelayConfig maps EventType strings to their outbox relay mappings.
// Events whose type is not present in the map are not forwarded.
type RelayConfig map[string]RelayMapping

// relayRepository wraps a base Repository and automatically enqueues matching
// events into the transactional outbox after each successful AppendEvent call.
type relayRepository struct {
	Repository
	outbox outbox.Repository
	config RelayConfig
}

// NewRelayRepository creates a Repository decorator that intercepts AppendEvent
// and copies the event to the outbox when its EventType matches a RelayConfig entry.
func NewRelayRepository(base Repository, outboxRepo outbox.Repository, config RelayConfig) Repository {
	if config == nil || len(config) == 0 {
		return base
	}
	return &relayRepository{Repository: base, outbox: outboxRepo, config: config}
}

func (r *relayRepository) AppendEvent(ctx context.Context, db *gorm.DB, event *DomainEvent) error {
	if err := r.Repository.AppendEvent(ctx, db, event); err != nil {
		return err
	}

	mapping, ok := r.config[event.EventType]
	if !ok {
		return nil // No relay mapping for this event type; append-only.
	}

	messageID := fmt.Sprintf("%s:%s", event.AggregateID, mapping.StepName)
	return r.outbox.Enqueue(ctx, db, outbox.EnqueueParams{
		MessageID:     messageID,
		Topic:         mapping.Topic,
		MessageKey:    event.AggregateID,
		MessageType:   mapping.MessageType,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		Payload:       []byte(event.Payload),
	})
}
