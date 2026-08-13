package query

import (
	"context"

	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type ValidateAPIKeyHandler struct {
	repo port.AgentRepository
}

func NewValidateAPIKeyHandler(repo port.AgentRepository) *ValidateAPIKeyHandler {
	return &ValidateAPIKeyHandler{repo: repo}
}

func (h *ValidateAPIKeyHandler) Handle(ctx context.Context, apiKey string) (*domain.AgentConfig, error) {
	return h.repo.GetConfigByApiKey(ctx, apiKey)
}
