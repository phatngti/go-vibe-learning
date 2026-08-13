package command

import (
	"context"
	"fmt"
	"strings"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type ConfigureCallbackHandler struct {
	uow  port.UnitOfWork
	repo port.AgentRepository
}

func NewConfigureCallbackHandler(uow port.UnitOfWork, repo port.AgentRepository) *ConfigureCallbackHandler {
	return &ConfigureCallbackHandler{uow: uow, repo: repo}
}

func (h *ConfigureCallbackHandler) Handle(ctx context.Context, cmd *dto.ConfigureCallback) (*domain.AgentConfig, error) {
	cfg, err := h.repo.GetConfig(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}

	cfg.CallbackURL = cmd.CallbackURL
	cfg.AllowedIPs = strings.Join(cmd.AllowedIPs, ",")

	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		if err := h.repo.SaveConfig(txCtx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID: cmd.AgentID,
			Action:  "CONFIGURE_CALLBACK",
			Details: fmt.Sprintf("URL: %s, IPs: %s", cmd.CallbackURL, cfg.AllowedIPs),
		}

		return h.repo.CreateAuditLog(txCtx, audit)
	})

	if err != nil {
		return nil, err
	}

	return cfg, nil
}
