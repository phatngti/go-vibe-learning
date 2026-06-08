package postgres

import (
	"context"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/payment/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *platformpostgres.DB) domain.TransactionRepository {
	return &transactionRepository{db: db.DB}
}

func (r *transactionRepository) Save(ctx context.Context, db *gorm.DB, tx *domain.Transaction) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	if err := activeDB.WithContext(ctx).Create(tx).Error; err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return r.GetByIDWithDB(ctx, r.db, id)
}

func (r *transactionRepository) GetByIDWithDB(ctx context.Context, db *gorm.DB, id string) (*domain.Transaction, error) {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}

	var tx domain.Transaction
	if err := activeDB.WithContext(ctx).First(&tx, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("failed to find transaction %s: %w", id, err)
	}
	return &tx, nil
}

func (r *transactionRepository) UpdateStatus(ctx context.Context, db *gorm.DB, id string, status domain.TransactionState) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	err := activeDB.WithContext(ctx).
		Model(&domain.Transaction{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}
	return nil
}
