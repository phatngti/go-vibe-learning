package query

import (
	"context"

	"github/master_go_cqrs/services/affiliate/internal/application/port"
)

type GetDownlineHandler struct {
	repo port.AffiliateRepository
}

func NewGetDownlineHandler(repo port.AffiliateRepository) *GetDownlineHandler {
	return &GetDownlineHandler{repo: repo}
}

func (h *GetDownlineHandler) Handle(ctx context.Context, agentID string) ([]string, error) {
	return h.repo.GetDownline(ctx, agentID)
}
