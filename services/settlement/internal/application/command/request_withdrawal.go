package command

import (
	"context"
	"fmt"

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

		if bal.ActiveBalance-cmd.Amount < -bal.CreditLimit {
			return fmt.Errorf("insufficient funds including credit limit (max withdrawal: %.2f)", bal.ActiveBalance+bal.CreditLimit)
		}

		bal.ActiveBalance -= cmd.Amount
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
