package port

import (
	"context"

	"github/master_go_cqrs/services/affiliate/internal/domain"
)

// AffiliateRepository provides access to referrals, agent metrics, and affiliate levels.
type AffiliateRepository interface {
	SaveReferral(ctx context.Context, ref *domain.Referral) error
	GetDownline(ctx context.Context, agentID string) ([]string, error)
	GetMetrics(ctx context.Context, agentID string) (*domain.AgentMetric, error)
	GetMetricsForUpdate(ctx context.Context, agentID string) (*domain.AgentMetric, error)
	SaveMetrics(ctx context.Context, metric *domain.AgentMetric) error
	GetLevel(ctx context.Context, agentID string) (*domain.AffiliateLevel, error)
	GetLevelForUpdate(ctx context.Context, agentID string) (*domain.AffiliateLevel, error)
	SaveLevel(ctx context.Context, lvl *domain.AffiliateLevel) error
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
