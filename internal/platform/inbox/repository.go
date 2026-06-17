// File: internal/platform/inbox/repository.go
// Description: GORM inbox repository to begin message transactions and mark them processed.

package inbox

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Begin(ctx context.Context, db *gorm.DB, messageID, topic, messageType string) (bool, error)
	MarkProcessed(ctx context.Context, db *gorm.DB, messageID string) error
}

type repository struct{}

func NewRepository() Repository {
	return repository{}
}

func (repository) Begin(ctx context.Context, db *gorm.DB, messageID, topic, messageType string) (bool, error) {
	row := &Message{
		MessageID:   messageID,
		Topic:       topic,
		MessageType: messageType,
		Status:      "PROCESSING",
		CreatedAt:   time.Now(),
	}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, fmt.Errorf("inbox: begin message %s: %w", messageID, result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (repository) MarkProcessed(ctx context.Context, db *gorm.DB, messageID string) error {
	now := time.Now()
	if err := db.WithContext(ctx).Model(&Message{}).
		Where("message_id = ?", messageID).
		Updates(map[string]any{"status": "PROCESSED", "processed_at": &now}).Error; err != nil {
		return fmt.Errorf("inbox: mark message %s processed: %w", messageID, err)
	}
	return nil
}
