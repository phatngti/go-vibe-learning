package command

import (
	"context"
	"fmt"

	"github/master_go_cqrs/contracts/messagebus"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
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

	if bal.CreditLimitMoney != nil {
		limit, err := sharedmoney.FromLegacyFloat64(cmd.Limit, bal.CreditLimitMoney.Currency)
		if err != nil {
			return nil, fmt.Errorf("invalid credit limit money: %w", err)
		}
		if limit.AmountMinor < 0 {
			return nil, fmt.Errorf("invalid credit limit money: amount must not be negative")
		}
		legacyLimit, err := messagebus.LegacyMajorFloat(limit)
		if err != nil {
			return nil, err
		}
		bal.CreditLimit = legacyLimit
		bal.CreditLimitMoney = messagebus.MoneyPointer(limit)
	} else {
		bal.CreditLimit = cmd.Limit
	}
	if err := h.repo.SaveBalance(ctx, bal); err != nil {
		return nil, err
	}

	return bal, nil
}
