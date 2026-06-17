package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/agent/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type agentRepository struct {
	db *gorm.DB
}

func NewAgentRepository(db *platformpostgres.DB) domain.AgentRepository {
	return &agentRepository{db: db.DB}
}

func (r *agentRepository) GetConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error) {
	var cfg domain.AgentConfig
	err := r.db.WithContext(ctx).First(&cfg, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentConfig{AgentID: agentID, CallbackURL: "", ApiKey: "", AllowedIPs: ""}, nil
		}
		return nil, fmt.Errorf("failed to load agent config: %w", err)
	}
	return &cfg, nil
}

func (r *agentRepository) GetConfigByApiKey(ctx context.Context, apiKey string) (*domain.AgentConfig, error) {
	var cfg domain.AgentConfig
	err := r.db.WithContext(ctx).First(&cfg, "api_key = ?", apiKey).Error
	if err != nil {
		return nil, fmt.Errorf("failed to locate agent by API key: %w", err)
	}
	return &cfg, nil
}

func (r *agentRepository) SaveConfig(ctx context.Context, db *gorm.DB, cfg *domain.AgentConfig) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	cfg.UpdatedAt = time.Now()
	if err := activeDB.WithContext(ctx).Save(cfg).Error; err != nil {
		return fmt.Errorf("failed to save agent config: %w", err)
	}
	return nil
}

func (r *agentRepository) CreateAuditLog(ctx context.Context, db *gorm.DB, log *domain.AuditLog) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	log.CreatedAt = time.Now()
	if err := activeDB.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to save audit log: %w", err)
	}
	return nil
}

func (r *agentRepository) GetCallbackDelivery(ctx context.Context, transactionID string) (*domain.CallbackDelivery, error) {
	var delivery domain.CallbackDelivery
	err := r.db.WithContext(ctx).First(&delivery, "transaction_id = ?", transactionID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load callback delivery: %w", err)
	}
	return &delivery, nil
}

func (r *agentRepository) SaveCallbackDelivery(ctx context.Context, db *gorm.DB, delivery *domain.CallbackDelivery) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	delivery.UpdatedAt = time.Now()
	if err := activeDB.WithContext(ctx).Save(delivery).Error; err != nil {
		return fmt.Errorf("failed to save callback delivery: %w", err)
	}
	return nil
}
