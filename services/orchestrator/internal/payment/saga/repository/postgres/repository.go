package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	paymentsaga "github/master_go_cqrs/services/orchestrator/internal/payment/saga/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *platformpostgres.DB) *Repository {
	return &Repository{db: db.DB}
}

func (r *Repository) GetForUpdate(ctx context.Context, tx *gorm.DB, transactionID string) (*paymentsaga.Model, error) {
	var model paymentsaga.Model
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&model, "transaction_id = ?", transactionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("orchestrator: load payment process %s: %w", transactionID, err)
	}
	return &model, nil
}

func (r *Repository) Save(ctx context.Context, tx *gorm.DB, model *paymentsaga.Model) error {
	model.UpdatedAt = time.Now()
	if err := tx.WithContext(ctx).Save(model).Error; err != nil {
		return fmt.Errorf("orchestrator: save payment process %s: %w", model.TransactionID, err)
	}
	return nil
}

func (r *Repository) DueIDs(ctx context.Context, limit int) ([]string, error) {
	var transactionIDs []string
	err := r.db.WithContext(ctx).Model(&paymentsaga.Model{}).
		Where("state IN ? AND next_retry_at <= ?", []payment.State{
			payment.StateWaitingAffiliate, payment.StateWaitingSettlement, payment.StateWaitingCallback,
		}, time.Now()).Order("next_retry_at").Limit(limit).Pluck("transaction_id", &transactionIDs).Error
	if err != nil {
		return nil, fmt.Errorf("orchestrator: list due payment processes: %w", err)
	}
	return transactionIDs, nil
}

func (r *Repository) DB() *gorm.DB { return r.db }
