package command

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type GenerateAPIKeyHandler struct {
	uow  port.UnitOfWork
	repo port.AgentRepository
}

func NewGenerateAPIKeyHandler(uow port.UnitOfWork, repo port.AgentRepository) *GenerateAPIKeyHandler {
	return &GenerateAPIKeyHandler{uow: uow, repo: repo}
}

func (h *GenerateAPIKeyHandler) Handle(ctx context.Context, cmd *dto.GenerateAPIKey) (*domain.AgentConfig, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random API key: %w", err)
	}
	apiKey := "agg_key_" + hex.EncodeToString(bytes)

	var configured *domain.AgentConfig
	err := h.uow.Do(ctx, func(txCtx context.Context) error {
		cfg, err := h.repo.GetConfig(txCtx, cmd.AgentID)
		if err != nil {
			return err
		}
		if cfg.TenantID != "" && cfg.TenantID != cmd.TenantID {
			return domain.ErrTenantOwnership
		}
		cfg.TenantID = cmd.TenantID
		cfg.ApiKey = apiKey
		if err := h.repo.SaveConfig(txCtx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID:  cmd.AgentID,
			TenantID: cmd.TenantID,
			Action:   "GENERATE_API_KEY",
			Details:  "Issued new API key credentials",
		}

		if err := h.repo.CreateAuditLog(txCtx, audit); err != nil {
			return err
		}
		configured = cfg
		return nil
	})

	if err != nil {
		return nil, err
	}

	return configured, nil
}
