package outbox

import (
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AggregateType string    `gorm:"type:text;not null"`
	AggregateID   string    `gorm:"type:text;not null"`
	EventType    string    `gorm:"type:text;not null"`
	Payload      string    `gorm:"type:text;not null"`
	Status       string    `gorm:"type:text;not null;default:'PENDING'"` // PENDING, PROCESSED, FAILED
	ErrorLog     string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"not null;default:now()"`
	ProcessedAt  *time.Time
}

func (OutboxEvent) TableName() string {
	return "public.outbox_events"
}
