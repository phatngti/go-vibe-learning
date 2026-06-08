package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	SaveEvent(ctx context.Context, db *gorm.DB, aggregateType, aggregateID, eventType, payload string) error
	GetPendingEvents(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkAsProcessed(ctx context.Context, id uuid.UUID) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errMsg string) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) SaveEvent(ctx context.Context, db *gorm.DB, aggregateType, aggregateID, eventType, payload string) error {
	// If a transaction DB is passed, use it; otherwise use default DB
	activeDB := r.db
	if db != nil {
		activeDB = db
	}

	event := &OutboxEvent{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		Status:        "PENDING",
		CreatedAt:     time.Now(),
	}

	if err := activeDB.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}
	return nil
}

func (r *gormRepository) GetPendingEvents(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	var events []*OutboxEvent
	err := r.db.WithContext(ctx).
		Where("status = ?", "PENDING").
		Order("created_at asc").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox events: %w", err)
	}
	return events, nil
}

func (r *gormRepository) MarkAsProcessed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       "PROCESSED",
			"processed_at": &now,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to mark outbox event %s as processed: %w", id, err)
	}
	return nil
}

func (r *gormRepository) MarkAsFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	err := r.db.WithContext(ctx).
		Model(&OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    "FAILED",
			"error_log": errMsg,
		}).Error
	if err != nil {
		return fmt.Errorf("failed to mark outbox event %s as failed: %w", id, err)
	}
	return nil
}
