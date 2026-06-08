package command

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github/master_go_cqrs/internal/agent/application/dto"
	"github/master_go_cqrs/internal/agent/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type GenerateAPIKeyHandler struct {
	db   *gorm.DB
	repo domain.AgentRepository
}

func NewGenerateAPIKeyHandler(db *platformpostgres.DB, repo domain.AgentRepository) *GenerateAPIKeyHandler {
	return &GenerateAPIKeyHandler{db: db.DB, repo: repo}
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

	err = h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.SaveConfig(ctx, dbTx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID: cmd.AgentID,
			Action:  "GENERATE_API_KEY",
			Details: "Issued new API key credentials",
		}

		if err := h.repo.CreateAuditLog(ctx, dbTx, audit); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return cfg, nil
}
