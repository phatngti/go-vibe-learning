// File: internal/platform/eventstore/models.go
// Description: Defines the DomainEvent model representing domain event log entries.

package eventstore

import (
	"time"

	"github.com/google/uuid"
)

// DomainEvent is an append-only record of something that happened to an aggregate.
// It is stored in the domain_events table and never mutated.
type DomainEvent struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AggregateType string    `gorm:"type:text;not null;index:idx_domain_events_aggregate"`
	AggregateID   string    `gorm:"type:text;not null;index:idx_domain_events_aggregate"`
	EventType     string    `gorm:"type:text;not null"`
	Payload       string    `gorm:"type:text;not null"`
	// Version is monotonically increasing per (aggregate_type, aggregate_id).
	// The UNIQUE constraint on (aggregate_type, aggregate_id, version) provides
	// optimistic concurrency: two concurrent writers on the same version will fail.
	Version    int64     `gorm:"not null;uniqueIndex:idx_domain_events_version"`
	OccurredAt time.Time `gorm:"not null;default:now()"`
}

func (DomainEvent) TableName() string {
	return "public.domain_events"
}
