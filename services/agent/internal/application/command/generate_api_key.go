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
	cfg, err := h.repo.GetConfig(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random API key: %w", err)
	}
	cfg.ApiKey = "agg_key_" + hex.EncodeToString(bytes)

	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		if err := h.repo.SaveConfig(txCtx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID: cmd.AgentID,
			Action:  "GENERATE_API_KEY",
			Details: "Issued new API key credentials",
		}

		return h.repo.CreateAuditLog(txCtx, audit)
	})

	if err != nil {
		return nil, err
	}

	return cfg, nil
}
