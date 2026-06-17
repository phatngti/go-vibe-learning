// File: internal/platform/outbox/models.go
// Description: Outbox event schema models and enqueue parameter structures.

package outbox

import (
	"time"

	"github.com/google/uuid"
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
	Status        string    `gorm:"type:text;not null;default:'PENDING'"`
	Attempts      int       `gorm:"not null;default:0"`
	AvailableAt   time.Time `gorm:"not null;default:now()"`
	LockedAt      *time.Time
	LockedBy      string `gorm:"type:text"`
	ErrorLog      string `gorm:"type:text"`
	CreatedAt     time.Time
	ProcessedAt   *time.Time
}

func (Event) TableName() string {
	return "public.outbox_events"
}

type EnqueueParams struct {
	MessageID     string
	Topic         string
	MessageKey    string
	MessageType   string
	AggregateType string
	AggregateID   string
	Payload       []byte
}
