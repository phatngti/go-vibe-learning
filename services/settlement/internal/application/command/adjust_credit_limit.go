package command

import (
	"context"

	"github/master_go_cqrs/services/settlement/internal/application/dto"
	"github/master_go_cqrs/services/settlement/internal/application/port"
	"github/master_go_cqrs/services/settlement/internal/domain"
)

type AdjustCreditLimitHandler struct {
	repo port.SettlementRepository
}

func NewAdjustCreditLimitHandler(repo port.SettlementRepository) *AdjustCreditLimitHandler {
	return &AdjustCreditLimitHandler{repo: repo}
}

func (h *AdjustCreditLimitHandler) Handle(ctx context.Context, cmd *dto.AdjustCreditLimit) (*domain.AgentBalance, error) {
	bal, err := h.repo.GetBalance(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}

	bal.CreditLimit = cmd.Limit
	if err := h.repo.SaveBalance(ctx, bal); err != nil {
		return nil, err
	}

	return bal, nil
}
