package command

import (
	"context"
	"fmt"
	"strings"

	"github/master_go_cqrs/internal/agent/application/dto"
	"github/master_go_cqrs/internal/agent/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type ConfigureCallbackHandler struct {
	db   *gorm.DB
	repo domain.AgentRepository
}

func NewConfigureCallbackHandler(db *platformpostgres.DB, repo domain.AgentRepository) *ConfigureCallbackHandler {
	return &ConfigureCallbackHandler{db: db.DB, repo: repo}
}

func (h *ConfigureCallbackHandler) Handle(ctx context.Context, cmd *dto.ConfigureCallback) (*domain.AgentConfig, error) {
	cfg, err := h.repo.GetConfig(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}

	cfg.CallbackURL = cmd.CallbackURL
	cfg.AllowedIPs = strings.Join(cmd.AllowedIPs, ",")

	err = h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.SaveConfig(ctx, dbTx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID: cmd.AgentID,
			Action:  "CONFIGURE_CALLBACK",
			Details: fmt.Sprintf("URL: %s, IPs: %s", cmd.CallbackURL, cfg.AllowedIPs),
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
