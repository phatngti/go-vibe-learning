package port

import (
	"context"

	"github/master_go_cqrs/services/agent/internal/domain"
)

// AgentRepository provides access to agent configuration, audit logs, and callback deliveries.
type AgentRepository interface {
	GetConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error)
	GetConfigByApiKey(ctx context.Context, apiKey string) (*domain.AgentConfig, error)
	SaveConfig(ctx context.Context, cfg *domain.AgentConfig) error
	CreateAuditLog(ctx context.Context, log *domain.AuditLog) error
	GetCallbackDelivery(ctx context.Context, transactionID string) (*domain.CallbackDelivery, error)
	SaveCallbackDelivery(ctx context.Context, delivery *domain.CallbackDelivery) error
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
