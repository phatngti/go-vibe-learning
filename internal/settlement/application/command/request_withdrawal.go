package command

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/domain"

	"gorm.io/gorm"
)

type RequestWithdrawalHandler struct {
	db   *gorm.DB
	repo domain.SettlementRepository
}

func NewRequestWithdrawalHandler(db *postgres.DB, repo domain.SettlementRepository) *RequestWithdrawalHandler {
	return &RequestWithdrawalHandler{db: db.DB, repo: repo}
}

func (h *RequestWithdrawalHandler) Handle(ctx context.Context, cmd *dto.RequestWithdrawal) (*domain.AgentBalance, error) {
	var updatedBalance *domain.AgentBalance

	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		bal, err := h.repo.GetBalance(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		if bal.ActiveBalance-cmd.Amount < -bal.CreditLimit {
			return fmt.Errorf("insufficient funds including credit limit (max withdrawal: %.2f)", bal.ActiveBalance+bal.CreditLimit)
		}

		bal.ActiveBalance -= cmd.Amount
		if err := h.repo.SaveBalance(ctx, dbTx, bal); err != nil {
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
