// File: internal/platform/inbox/models.go
// Description: Defines the Message struct representing inbox events for idempotent message processing.

package inbox

import "time"

type Message struct {
	MessageID   string    `gorm:"type:text;primaryKey"`
	Topic       string    `gorm:"type:text;not null"`
	MessageType string    `gorm:"type:text;not null"`
	Status      string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
	ProcessedAt *time.Time
}

func (Message) TableName() string {
	return "public.inbox_messages"
}
