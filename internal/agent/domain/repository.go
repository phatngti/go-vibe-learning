package domain

import (
	"context"

	"gorm.io/gorm"
)

type AgentRepository interface {
	GetConfig(ctx context.Context, agentID string) (*AgentConfig, error)
	GetConfigByApiKey(ctx context.Context, apiKey string) (*AgentConfig, error)
	SaveConfig(ctx context.Context, db *gorm.DB, cfg *AgentConfig) error
	CreateAuditLog(ctx context.Context, db *gorm.DB, log *AuditLog) error
	GetCallbackDelivery(ctx context.Context, transactionID string) (*CallbackDelivery, error)
	SaveCallbackDelivery(ctx context.Context, db *gorm.DB, delivery *CallbackDelivery) error
}
