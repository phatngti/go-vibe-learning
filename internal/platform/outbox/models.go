// File: internal/platform/outbox/models.go
// Description: Outbox event schema models and enqueue parameter structures.

package outbox

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusProcessed  = "PROCESSED"
	StatusDead       = "DEAD"
)

type Event struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	MessageID     string    `gorm:"type:text;not null;uniqueIndex"`
	Topic         string    `gorm:"type:text;not null"`
	MessageKey    string    `gorm:"type:text;not null"`
	MessageType   string    `gorm:"type:text;not null"`
	AggregateType string    `gorm:"type:text;not null"`
	AggregateID   string    `gorm:"type:text;not null"`
	Payload       string    `gorm:"type:text;not null"`
	// Envelope metadata is read from deployments that have applied the optional
	// metadata columns. Enqueue writes it explicitly after the core row insert so
	// services can adopt the shared migration independently.
	SchemaVersion           int       `gorm:"column:schema_version;->"`
	AggregateVersion        int64     `gorm:"column:aggregate_version;->"`
	TenantID                string    `gorm:"column:tenant_id;->"`
	CorrelationID           string    `gorm:"column:correlation_id;->"`
	CausationID             string    `gorm:"column:causation_id;->"`
	OrchestrationOwner      string    `gorm:"column:orchestration_owner;->"`
	OrchestrationGeneration int64     `gorm:"column:orchestration_generation;->"`
	TraceParent             string    `gorm:"column:traceparent;->"`
	TraceState              string    `gorm:"column:tracestate;->"`
	Status                  string    `gorm:"type:text;not null;default:'PENDING'"`
	Attempts                int       `gorm:"not null;default:0"`
	AvailableAt             time.Time `gorm:"not null;default:now()"`
	LockedAt                *time.Time
	LockedBy                string `gorm:"type:text"`
	ErrorLog                string `gorm:"type:text"`
	CreatedAt               time.Time
	ProcessedAt             *time.Time
	DeadAt                  *time.Time
}

func (Event) TableName() string {
	return "public.outbox_events"
}

type EnqueueParams struct {
	MessageID               string
	Topic                   string
	MessageKey              string
	MessageType             string
	AggregateType           string
	AggregateID             string
	Payload                 []byte
	SchemaVersion           int
	AggregateVersion        int64
	TenantID                string
	CorrelationID           string
	CausationID             string
	OrchestrationOwner      string
	OrchestrationGeneration int64
	TraceParent             string
	TraceState              string
}

// Stats is a low-cardinality operational snapshot used for SLO dashboards.
type Stats struct {
	PendingCount     int64
	ProcessingCount  int64
	DeadCount        int64
	OldestPendingAge time.Duration
}
