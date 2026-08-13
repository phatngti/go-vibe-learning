package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/settlement/internal/application/port"
	"github/master_go_cqrs/services/settlement/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type settlementRepository struct{ db *gorm.DB }

func NewSettlementRepository(db *platformpostgres.DB) port.SettlementRepository {
	return &settlementRepository{db: db.DB}
}

func (r *settlementRepository) GetBalance(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	return r.getBalance(ctx, r.db, agentID, false)
}

func (r *settlementRepository) GetBalanceForUpdate(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	return r.getBalance(ctx, activeDB, agentID, true)
}

func (r *settlementRepository) getBalance(ctx context.Context, db *gorm.DB, agentID string, lock bool) (*domain.AgentBalance, error) {
	var row AgentBalanceRow
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&row, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentBalance{AgentID: agentID, ActiveBalance: 0, CreditLimit: 1000}, nil
		}
		return nil, fmt.Errorf("failed to load agent balance: %w", err)
	}
	return balanceToDomain(&row), nil
}

func (r *settlementRepository) SaveBalance(ctx context.Context, bal *domain.AgentBalance) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	bal.UpdatedAt = time.Now()
	row := balanceToRow(bal)
	if err := activeDB.WithContext(ctx).Save(row).Error; err != nil {
		return fmt.Errorf("failed to save agent balance: %w", err)
	}
	return nil
}

func (r *settlementRepository) CreateCommissionLog(ctx context.Context, log *domain.CommissionLog) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	log.CreatedAt = time.Now()
	row := commissionLogToRow(log)
	if err := activeDB.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("failed to save commission log: %w", err)
	}
	log.ID = row.ID
	return nil
}
