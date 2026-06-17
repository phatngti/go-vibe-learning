package saga

import (
	"context"

	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"

	"gorm.io/gorm"
)

// EventProcessor owns the feature-specific state transitions for a long-running process.
type EventProcessor interface {
	TopicFor(eventType string) string
	ProcessEvent(ctx context.Context, tx *gorm.DB, message *kafka.Message) error
}

// Handler provides transactional inbox deduplication around a feature event processor.
type Handler struct {
	db        *gorm.DB
	inbox     inbox.Repository
	processor EventProcessor
}

func NewHandler(db *gorm.DB, inboxRepo inbox.Repository, processor EventProcessor) *Handler {
	return &Handler{db: db, inbox: inboxRepo, processor: processor}
}

func (h *Handler) Handle(ctx context.Context, message *kafka.Message) error {
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, message.ID, h.processor.TopicFor(message.EventType), message.EventType)
		if err != nil || !fresh {
			return err
		}
		if err := h.processor.ProcessEvent(ctx, tx, message); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, message.ID)
	})
}
