// File: internal/platform/eventstore/repository.go
// Description: GORM repository implementation for appending and loading aggregate domain events.

package eventstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides append-only access to the domain event log.
type Repository interface {
	// AppendEvent appends a single domain event atomically. If a db transaction
	// is provided it participates in it; otherwise the default DB is used.
	// Returns an error if the (aggregate_type, aggregate_id, version) triple
	// already exists (optimistic concurrency violation).
	AppendEvent(ctx context.Context, db *gorm.DB, event *DomainEvent) error

	// LoadEvents returns all domain events for an aggregate ordered by version.
	LoadEvents(ctx context.Context, aggregateType, aggregateID string) ([]*DomainEvent, error)

	// LoadEventsFrom returns domain events starting from (and including) fromVersion.
	LoadEventsFrom(ctx context.Context, aggregateType, aggregateID string, fromVersion int64) ([]*DomainEvent, error)

	// NextVersion returns version = max(version)+1 for an aggregate, or 1 if no events exist.
	NextVersion(ctx context.Context, db *gorm.DB, aggregateType, aggregateID string) (int64, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository creates a new eventstore Repository backed by GORM.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) AppendEvent(ctx context.Context, db *gorm.DB, event *DomainEvent) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	if db != nil {
		activeDB = db
	}

	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}

	if err := activeDB.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("eventstore: failed to append event %s for %s/%s@v%d: %w",
			event.EventType, event.AggregateType, event.AggregateID, event.Version, err)
	}
	return nil
}

func (r *gormRepository) LoadEvents(ctx context.Context, aggregateType, aggregateID string) ([]*DomainEvent, error) {
	return r.LoadEventsFrom(ctx, aggregateType, aggregateID, 0)
}

func (r *gormRepository) LoadEventsFrom(ctx context.Context, aggregateType, aggregateID string, fromVersion int64) ([]*DomainEvent, error) {
	var events []*DomainEvent
	err := r.db.WithContext(ctx).
		Where("aggregate_type = ? AND aggregate_id = ? AND version >= ?", aggregateType, aggregateID, fromVersion).
		Order("version asc").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("eventstore: failed to load events for %s/%s: %w", aggregateType, aggregateID, err)
	}
	return events, nil
}

func (r *gormRepository) NextVersion(ctx context.Context, db *gorm.DB, aggregateType, aggregateID string) (int64, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	if db != nil {
		activeDB = db
	}

	var maxVersion int64
	err := activeDB.WithContext(ctx).
		Model(&DomainEvent{}).
		Where("aggregate_type = ? AND aggregate_id = ?", aggregateType, aggregateID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("eventstore: failed to query max version for %s/%s: %w", aggregateType, aggregateID, err)
	}
	return maxVersion + 1, nil
}
