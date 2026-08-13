package port

import (
	"context"

	"github/master_go_cqrs/services/settlement/internal/domain"
)

// SettlementRepository provides access to balances and commission ledger.
type SettlementRepository interface {
	GetBalance(ctx context.Context, agentID string) (*domain.AgentBalance, error)
	GetBalanceForUpdate(ctx context.Context, agentID string) (*domain.AgentBalance, error)
	SaveBalance(ctx context.Context, bal *domain.AgentBalance) error
	CreateCommissionLog(ctx context.Context, log *domain.CommissionLog) error
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
