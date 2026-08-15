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

type RequestWithdrawalHandler struct {
	uow  port.UnitOfWork
	repo port.SettlementRepository
}

func NewRequestWithdrawalHandler(uow port.UnitOfWork, repo port.SettlementRepository) *RequestWithdrawalHandler {
	return &RequestWithdrawalHandler{uow: uow, repo: repo}
}

func (h *RequestWithdrawalHandler) Handle(ctx context.Context, cmd *dto.RequestWithdrawal) (*domain.AgentBalance, error) {
	var updatedBalance *domain.AgentBalance

	err := h.uow.Do(ctx, func(txCtx context.Context) error {
		bal, err := h.repo.GetBalance(txCtx, cmd.AgentID)
		if err != nil {
			return err
		}

		if bal.ActiveBalanceMoney != nil && bal.CreditLimitMoney != nil {
			withdrawal, err := sharedmoney.FromLegacyFloat64(cmd.Amount, bal.ActiveBalanceMoney.Currency)
			if err != nil {
				return fmt.Errorf("invalid withdrawal money: %w", err)
			}
			if !withdrawal.IsPositive() {
				return fmt.Errorf("invalid withdrawal money: amount must be positive")
			}
			active, err := sharedmoney.New(bal.ActiveBalanceMoney.AmountMinor, bal.ActiveBalanceMoney.Currency, bal.ActiveBalanceMoney.Exponent)
			if err != nil {
				return err
			}
			credit, err := sharedmoney.New(bal.CreditLimitMoney.AmountMinor, bal.CreditLimitMoney.Currency, bal.CreditLimitMoney.Exponent)
			if err != nil {
				return fmt.Errorf("invalid credit limit money: %w", err)
			}
			if credit.AmountMinor < 0 {
				return fmt.Errorf("invalid credit limit money: amount must not be negative")
			}
			remaining, err := active.Subtract(withdrawal)
			if err != nil {
				return err
			}
			if remaining.AmountMinor < -credit.AmountMinor {
				return domain.ErrInsufficientBalance
			}
			legacyRemaining, err := messagebus.LegacyMajorFloat(remaining)
			if err != nil {
				return err
			}
			bal.ActiveBalance = legacyRemaining
			bal.ActiveBalanceMoney = messagebus.MoneyPointer(remaining)
		} else {
			if bal.ActiveBalance-cmd.Amount < -bal.CreditLimit {
				return fmt.Errorf("insufficient funds including credit limit (max withdrawal: %.2f)", bal.ActiveBalance+bal.CreditLimit)
			}
			bal.ActiveBalance -= cmd.Amount
		}
		if err := h.repo.SaveBalance(txCtx, bal); err != nil {
			return err
		}

		updatedBalance = bal
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedBalance, nil
}
