package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentRepository struct{ db *gorm.DB }

func NewAgentRepository(db *platformpostgres.DB) port.AgentRepository {
	return &agentRepository{db: db.DB}
}

func (r *agentRepository) GetConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error) {
	var row AgentConfigRow
	activeDB := platformpostgres.FromContext(ctx, r.db)
	err := activeDB.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentConfig{AgentID: agentID, CallbackStatus: domain.CallbackStatusPending}, nil
		}
		return nil, fmt.Errorf("failed to load agent config: %w", err)
	}
	return configToDomain(&row), nil
}

func (r *agentRepository) GetConfigByApiKey(ctx context.Context, apiKey string) (*domain.AgentConfig, error) {
	var row AgentConfigRow
	err := r.db.WithContext(ctx).First(&row, "api_key = ?", apiKey).Error
	if err != nil {
		return nil, fmt.Errorf("failed to locate agent by API key: %w", err)
	}
	return configToDomain(&row), nil
}

func (r *agentRepository) SaveConfig(ctx context.Context, cfg *domain.AgentConfig) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	cfg.UpdatedAt = time.Now()
	row := configToRow(cfg)
	if err := activeDB.WithContext(ctx).Save(row).Error; err != nil {
		return fmt.Errorf("failed to save agent config: %w", err)
	}
	return nil
}

func (r *agentRepository) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	log.CreatedAt = time.Now()
	row := auditLogToRow(log)
	if err := activeDB.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("failed to save audit log: %w", err)
	}
	log.ID = row.ID
	return nil
}
