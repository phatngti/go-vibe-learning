package command

import (
	"context"

	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/domain"
)

type AdjustCreditLimitHandler struct {
	repo domain.SettlementRepository
}

func NewAdjustCreditLimitHandler(repo domain.SettlementRepository) *AdjustCreditLimitHandler {
	return &AdjustCreditLimitHandler{repo: repo}
}

func (h *AdjustCreditLimitHandler) Handle(ctx context.Context, cmd *dto.AdjustCreditLimit) (*domain.AgentBalance, error) {
	bal, err := h.repo.GetBalance(ctx, cmd.AgentID)
	if err != nil {
		return nil, err
	}

	bal.CreditLimit = cmd.Limit
	if err := h.repo.SaveBalance(ctx, nil, bal); err != nil {
		return nil, err
	}

	return bal, nil
}
