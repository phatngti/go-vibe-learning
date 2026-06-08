package domain

import (
	"context"

	"gorm.io/gorm"
)

type SettlementRepository interface {
	GetBalance(ctx context.Context, agentID string) (*AgentBalance, error)
	SaveBalance(ctx context.Context, db *gorm.DB, bal *AgentBalance) error
	CreateCommissionLog(ctx context.Context, db *gorm.DB, log *CommissionLog) error
}
