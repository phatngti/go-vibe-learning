package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/settlement/domain"

	"gorm.io/gorm"
)

type settlementRepository struct {
	db *gorm.DB
}

func NewSettlementRepository(db *platformpostgres.DB) domain.SettlementRepository {
	return &settlementRepository{db: db.DB}
}

func (r *settlementRepository) GetBalance(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	var bal domain.AgentBalance
	err := r.db.WithContext(ctx).First(&bal, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentBalance{AgentID: agentID, ActiveBalance: 0, CreditLimit: 1000}, nil
		}
		return nil, fmt.Errorf("failed to load agent balance: %w", err)
	}
	return &bal, nil
}

func (r *settlementRepository) SaveBalance(ctx context.Context, db *gorm.DB, bal *domain.AgentBalance) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	bal.UpdatedAt = time.Now()
	if err := activeDB.WithContext(ctx).Save(bal).Error; err != nil {
		return fmt.Errorf("failed to save agent balance: %w", err)
	}
	return nil
}

func (r *settlementRepository) CreateCommissionLog(ctx context.Context, db *gorm.DB, log *domain.CommissionLog) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	log.CreatedAt = time.Now()
	if err := activeDB.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to save commission log: %w", err)
	}
	return nil
}
