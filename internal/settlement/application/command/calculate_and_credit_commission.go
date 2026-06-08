package command

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/domain"

	"gorm.io/gorm"
)

type CalculateAndCreditCommissionHandler struct {
	db   *gorm.DB
	repo domain.SettlementRepository
}

func NewCalculateAndCreditCommissionHandler(db *postgres.DB, repo domain.SettlementRepository) *CalculateAndCreditCommissionHandler {
	return &CalculateAndCreditCommissionHandler{db: db.DB, repo: repo}
}

func (h *CalculateAndCreditCommissionHandler) Handle(ctx context.Context, cmd *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	rate := 0.01
	switch cmd.Rank {
	case "SILVER":
		rate = 0.02
	case "GOLD":
		rate = 0.03
	case "PLATINUM":
		rate = 0.04
	case "DIAMOND":
		rate = 0.05
	}

	commission := cmd.TransactionAmount * rate
	var updatedBalance *domain.AgentBalance

	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		bal, err := h.repo.GetBalance(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		bal.ActiveBalance += commission
		if err := h.repo.SaveBalance(ctx, dbTx, bal); err != nil {
			return err
		}

		log := &domain.CommissionLog{
			AgentID:           cmd.AgentID,
			TransactionID:     cmd.TransactionID,
			TransactionAmount: cmd.TransactionAmount,
			CommissionAmount:  commission,
			Rank:              cmd.Rank,
		}

		if err := h.repo.CreateCommissionLog(ctx, dbTx, log); err != nil {
			return err
		}

		updatedBalance = bal
		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to calculate and credit commission: %w", err)
	}

	return updatedBalance, commission, nil
}
