package eventstore

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

type BatchHandler func(context.Context, *gorm.DB, *DomainEvent) error

type ConsumerCheckpoint struct {
	ConsumerName  string    `gorm:"type:text;primaryKey"`
	LastEventTime time.Time `gorm:"not null"`
	LastEventID   uuid.UUID `gorm:"type:uuid;not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (ConsumerCheckpoint) TableName() string { return "public.event_consumers" }

type Consumer struct {
	db *gorm.DB
}

func NewConsumer(db *platformpostgres.DB) *Consumer {
	return &Consumer{db: db.DB}
}

// ConsumeBatch serializes work for a named consumer, invokes the handler in
// the same transaction, and advances its durable checkpoint after each event.
func (c *Consumer) ConsumeBatch(
	ctx context.Context,
	consumerName string,
	aggregateType string,
	eventTypes []string,
	limit int,
	handler BatchHandler,
) (int, error) {
	processed := 0
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		checkpoint := ConsumerCheckpoint{
			ConsumerName: consumerName, LastEventTime: time.Unix(0, 0).UTC(),
			LastEventID: uuid.Nil, UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&checkpoint).Error; err != nil {
			return fmt.Errorf("eventstore: create consumer checkpoint %q: %w", consumerName, err)
		}

		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			First(&checkpoint, "consumer_name = ?", consumerName)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("eventstore: lock consumer checkpoint %q: %w", consumerName, result.Error)
		}

		query := tx.Where("aggregate_type = ?", aggregateType).
			Where("(occurred_at > ?) OR (occurred_at = ? AND id > ?)", checkpoint.LastEventTime, checkpoint.LastEventTime, checkpoint.LastEventID)
		if len(eventTypes) > 0 {
			query = query.Where("event_type IN ?", eventTypes)
		}
		var events []*DomainEvent
		if err := query.Order("occurred_at ASC, id ASC").Limit(limit).Find(&events).Error; err != nil {
			return fmt.Errorf("eventstore: load events for consumer %q: %w", consumerName, err)
		}
		for _, event := range events {
			if err := handler(ctx, tx, event); err != nil {
				return fmt.Errorf("eventstore: consumer %q handle event %s: %w", consumerName, event.ID, err)
			}
			checkpoint.LastEventTime = event.OccurredAt
			checkpoint.LastEventID = event.ID
			checkpoint.UpdatedAt = time.Now()
			if err := tx.Save(&checkpoint).Error; err != nil {
				return fmt.Errorf("eventstore: advance consumer checkpoint %q: %w", consumerName, err)
			}
			processed++
		}
		return nil
	})
	return processed, err
}
