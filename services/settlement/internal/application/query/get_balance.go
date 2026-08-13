package query

import (
	"context"

	"github/master_go_cqrs/services/settlement/internal/application/port"
	"github/master_go_cqrs/services/settlement/internal/domain"
)

type GetBalanceHandler struct {
	repo port.SettlementRepository
}

func NewGetBalanceHandler(repo port.SettlementRepository) *GetBalanceHandler {
	return &GetBalanceHandler{repo: repo}
}

func (h *GetBalanceHandler) Handle(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	return h.repo.GetBalance(ctx, agentID)
}
