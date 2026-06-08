package domain

import (
	"context"

	"gorm.io/gorm"
)

type AffiliateRepository interface {
	SaveReferral(ctx context.Context, db *gorm.DB, ref *Referral) error
	GetDownline(ctx context.Context, agentID string) ([]string, error)
	GetMetrics(ctx context.Context, agentID string) (*AgentMetric, error)
	SaveMetrics(ctx context.Context, db *gorm.DB, metric *AgentMetric) error
	GetLevel(ctx context.Context, agentID string) (*AffiliateLevel, error)
	SaveLevel(ctx context.Context, db *gorm.DB, lvl *AffiliateLevel) error
}
