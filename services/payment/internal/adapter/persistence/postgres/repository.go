package postgres

import (
	"context"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"

	"gorm.io/gorm"
)

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *platformpostgres.DB) port.TransactionRepository {
	return &transactionRepository{db: db.DB}
}

func (r *transactionRepository) Save(ctx context.Context, tx *domain.Transaction) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := toRow(tx)
	if err := activeDB.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	// Write back generated ID
	tx.ID = row.ID
	tx.CreatedAt = row.CreatedAt
	tx.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row TransactionRow
	if err := activeDB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to find transaction %s: %w", id, err)
	}
	return toDomain(&row), nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, id string, status domain.TransactionState) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	err := activeDB.WithContext(ctx).
		Model(&TransactionRow{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now(),
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}
	return nil
}
