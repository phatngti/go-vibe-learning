package query

import (
	"context"

	"github/master_go_cqrs/internal/affiliate/domain"
)

type GetDownlineHandler struct {
	repo domain.AffiliateRepository
}

func NewGetDownlineHandler(repo domain.AffiliateRepository) *GetDownlineHandler {
	return &GetDownlineHandler{repo: repo}
}

func (h *GetDownlineHandler) Handle(ctx context.Context, agentID string) ([]string, error) {
	return h.repo.GetDownline(ctx, agentID)
}
