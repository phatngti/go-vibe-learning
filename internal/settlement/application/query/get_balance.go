package query

import (
	"context"

	"github/master_go_cqrs/internal/settlement/domain"
)

type GetBalanceHandler struct {
	repo domain.SettlementRepository
}

func NewGetBalanceHandler(repo domain.SettlementRepository) *GetBalanceHandler {
	return &GetBalanceHandler{repo: repo}
}

func (h *GetBalanceHandler) Handle(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	return h.repo.GetBalance(ctx, agentID)
}
