package query

import (
	"context"

	"github/master_go_cqrs/internal/affiliate/domain"
)

type GetRankHandler struct {
	repo domain.AffiliateRepository
}

func NewGetRankHandler(repo domain.AffiliateRepository) *GetRankHandler {
	return &GetRankHandler{repo: repo}
}

func (h *GetRankHandler) Handle(ctx context.Context, agentID string) (*domain.AffiliateLevel, *domain.AgentMetric, error) {
	level, err := h.repo.GetLevel(ctx, agentID)
	if err != nil {
		return nil, nil, err
	}
	metrics, err := h.repo.GetMetrics(ctx, agentID)
	if err != nil {
		return nil, nil, err
	}
	return level, metrics, nil
}
