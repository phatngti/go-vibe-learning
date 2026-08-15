// File: internal/platform/outbox/repository.go
// Description: GORM repository implementation to enqueue, claim, and update outbox events.

package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLeaseLost = errors.New("outbox processing lease lost")

type Repository interface {
	Enqueue(ctx context.Context, db *gorm.DB, params EnqueueParams) error
	Claim(ctx context.Context, workerID string, limit int, staleAfter time.Duration) ([]*Event, error)
	MarkProcessed(ctx context.Context, id uuid.UUID, workerID string) error
	MarkRetry(ctx context.Context, id uuid.UUID, workerID, errMsg string, delay time.Duration, maxAttempts int) error
	Stats(ctx context.Context) (Stats, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Enqueue(ctx context.Context, db *gorm.DB, params EnqueueParams) error {
	if (params.OrchestrationOwner == "") != (params.OrchestrationGeneration == 0) {
		return fmt.Errorf("outbox: orchestration owner and generation must be provided together")
	}
	if params.AggregateVersion < 0 || params.OrchestrationGeneration < 0 {
		return fmt.Errorf("outbox: envelope versions cannot be negative")
	}
	activeDB := platformpostgres.FromContext(ctx, r.db)
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
		Status:        StatusPending,
		AvailableAt:   now,
		CreatedAt:     now,
	}
	// A message ID represents one immutable logical message. Redelivery is handled
	// by Kafka/inbox idempotency; enqueueing the same ID must never mutate or
	// resurrect a message that was already published.
	result := activeDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}},
		DoNothing: true,
	}).Create(event)
	if result.Error != nil {
		return fmt.Errorf("outbox: enqueue message %s: %w", params.MessageID, result.Error)
	}
	metadata := envelopeMetadata(params)
	if result.RowsAffected == 1 && len(metadata) > 0 {
		if err := activeDB.WithContext(ctx).Table(event.TableName()).Where("id = ?", event.ID).Updates(metadata).Error; err != nil {
			return fmt.Errorf("outbox: persist envelope metadata for message %s: %w", params.MessageID, err)
		}
	}
	return nil
}

func envelopeMetadata(params EnqueueParams) map[string]any {
	if params.SchemaVersion == 0 && params.AggregateVersion == 0 && params.TenantID == "" &&
		params.CorrelationID == "" && params.CausationID == "" && params.OrchestrationOwner == "" &&
		params.OrchestrationGeneration == 0 && params.TraceParent == "" && params.TraceState == "" {
		return nil
	}
	schemaVersion := params.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = 1
	}
	return map[string]any{
		"schema_version": schemaVersion, "aggregate_version": params.AggregateVersion,
		"tenant_id": params.TenantID, "correlation_id": params.CorrelationID, "causation_id": params.CausationID,
		"orchestration_owner": params.OrchestrationOwner, "orchestration_generation": params.OrchestrationGeneration,
		"traceparent": params.TraceParent, "tracestate": params.TraceState,
	}
}

func (r *repository) Claim(ctx context.Context, workerID string, limit int, staleAfter time.Duration) ([]*Event, error) {
	var events []*Event
	staleBefore := time.Now().Add(-staleAfter)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
				WITH picked AS (
					SELECT events.id
					FROM public.outbox_events AS events
					WHERE ((events.status = 'PENDING' AND events.available_at <= NOW())
					   OR (events.status = 'PROCESSING' AND events.locked_at < ?))
					  AND NOT EXISTS (
						SELECT 1 FROM public.outbox_events AS prior
						WHERE prior.message_key = events.message_key
						  AND prior.status IN ('PENDING', 'PROCESSING', 'DEAD')
						  AND (prior.created_at < events.created_at
						       OR (prior.created_at = events.created_at AND prior.id::text < events.id::text))
					  )
					ORDER BY events.created_at, events.id
					FOR UPDATE OF events SKIP LOCKED
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

func (r *repository) MarkProcessed(ctx context.Context, id uuid.UUID, workerID string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&Event{}).
		Where("id = ? AND status = 'PROCESSING' AND locked_by = ?", id, workerID).
		Updates(map[string]any{
			"status": "PROCESSED", "processed_at": &now, "locked_at": nil,
			"locked_by": "", "error_log": "",
		})
	if result.Error != nil {
		return fmt.Errorf("outbox: mark event %s processed: %w", id, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox: mark event %s processed: %w", id, ErrLeaseLost)
	}
	return nil
}

func (r *repository) MarkRetry(ctx context.Context, id uuid.UUID, workerID, errMsg string, delay time.Duration, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = 20
	}
	now := time.Now()
	errMsg = truncateError(errMsg, 4096)
	result := r.db.WithContext(ctx).Exec(`
		UPDATE public.outbox_events
		SET attempts = attempts + 1,
		    status = CASE WHEN attempts + 1 >= ? THEN 'DEAD' ELSE 'PENDING' END,
		    available_at = CASE WHEN attempts + 1 >= ? THEN available_at ELSE ? END,
		    dead_at = CASE WHEN attempts + 1 >= ? THEN ? ELSE NULL END,
		    locked_at = NULL, locked_by = '', error_log = ?
		WHERE id = ? AND status = 'PROCESSING' AND locked_by = ?`, maxAttempts, maxAttempts, now.Add(delay), maxAttempts, now, errMsg, id, workerID)
	if result.Error != nil {
		return fmt.Errorf("outbox: reschedule event %s: %w", id, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox: reschedule event %s: %w", id, ErrLeaseLost)
	}
	return nil
}

func (r *repository) Stats(ctx context.Context) (Stats, error) {
	var row struct {
		PendingCount          int64
		ProcessingCount       int64
		DeadCount             int64
		OldestPendingAgeMilli float64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending_count,
			COUNT(*) FILTER (WHERE status = 'PROCESSING') AS processing_count,
			COUNT(*) FILTER (WHERE status = 'DEAD') AS dead_count,
			COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(created_at) FILTER (WHERE status = 'PENDING'))) * 1000, 0)
				AS oldest_pending_age_milli
		FROM public.outbox_events`).Scan(&row).Error
	if err != nil {
		return Stats{}, fmt.Errorf("outbox: load operational stats: %w", err)
	}
	return Stats{
		PendingCount: row.PendingCount, ProcessingCount: row.ProcessingCount, DeadCount: row.DeadCount,
		OldestPendingAge: time.Duration(row.OldestPendingAgeMilli * float64(time.Millisecond)),
	}, nil
}

func truncateError(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
