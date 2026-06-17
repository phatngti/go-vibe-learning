// File: internal/platform/outbox/repository.go
// Description: GORM repository implementation to enqueue, claim, and update outbox events.

package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Enqueue(ctx context.Context, db *gorm.DB, params EnqueueParams) error
	Claim(ctx context.Context, workerID string, limit int, staleAfter time.Duration) ([]*Event, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkRetry(ctx context.Context, id uuid.UUID, errMsg string, delay time.Duration) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Enqueue(ctx context.Context, db *gorm.DB, params EnqueueParams) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	now := time.Now()
	event := &Event{
		ID:            uuid.New(),
		MessageID:     params.MessageID,
		Topic:         params.Topic,
		MessageKey:    params.MessageKey,
		MessageType:   params.MessageType,
		AggregateType: params.AggregateType,
		AggregateID:   params.AggregateID,
		Payload:       string(params.Payload),
		Status:        "PENDING",
		AvailableAt:   now,
		CreatedAt:     now,
	}
	err := activeDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "message_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"topic": params.Topic, "message_key": params.MessageKey,
			"message_type": params.MessageType, "aggregate_type": params.AggregateType,
			"aggregate_id": params.AggregateID, "payload": string(params.Payload),
			"status": "PENDING", "available_at": now, "locked_at": nil,
			"locked_by": "", "error_log": "", "processed_at": nil,
		}),
	}).Create(event).Error
	if err != nil {
		return fmt.Errorf("outbox: enqueue message %s: %w", params.MessageID, err)
	}
	return nil
}

func (r *repository) Claim(ctx context.Context, workerID string, limit int, staleAfter time.Duration) ([]*Event, error) {
	var events []*Event
	staleBefore := time.Now().Add(-staleAfter)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
			WITH picked AS (
				SELECT id
				FROM public.outbox_events
				WHERE (status = 'PENDING' AND available_at <= NOW())
				   OR (status = 'PROCESSING' AND locked_at < ?)
				ORDER BY created_at
				FOR UPDATE SKIP LOCKED
				LIMIT ?
			)
			UPDATE public.outbox_events AS events
			SET status = 'PROCESSING', locked_at = NOW(), locked_by = ?
			FROM picked
			WHERE events.id = picked.id
			RETURNING events.*`, staleBefore, limit, workerID).Scan(&events).Error
	})
	if err != nil {
		return nil, fmt.Errorf("outbox: claim pending events: %w", err)
	}
	return events, nil
}

func (r *repository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	err := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Updates(map[string]any{
		"status": "PROCESSED", "processed_at": &now, "locked_at": nil,
		"locked_by": "", "error_log": "",
	}).Error
	if err != nil {
		return fmt.Errorf("outbox: mark event %s processed: %w", id, err)
	}
	return nil
}

func (r *repository) MarkRetry(ctx context.Context, id uuid.UUID, errMsg string, delay time.Duration) error {
	err := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Updates(map[string]any{
		"status": "PENDING", "attempts": gorm.Expr("attempts + 1"),
		"available_at": time.Now().Add(delay), "locked_at": nil,
		"locked_by": "", "error_log": errMsg,
	}).Error
	if err != nil {
		return fmt.Errorf("outbox: reschedule event %s: %w", id, err)
	}
	return nil
}
