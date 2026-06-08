package query

import (
	"context"

	"github/master_go_cqrs/internal/agent/domain"
)

type ValidateAPIKeyHandler struct {
	repo domain.AgentRepository
}

func NewValidateAPIKeyHandler(repo domain.AgentRepository) *ValidateAPIKeyHandler {
	return &ValidateAPIKeyHandler{repo: repo}
}

func (h *ValidateAPIKeyHandler) Handle(ctx context.Context, apiKey string) (*domain.AgentConfig, error) {
	return h.repo.GetConfigByApiKey(ctx, apiKey)
}
