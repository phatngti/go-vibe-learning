package command

import (
	"context"

	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/domain"
)

type InitiatePaymentHandler struct {
	repo domain.TransactionRepository
}

func NewInitiatePaymentHandler(repo domain.TransactionRepository) *InitiatePaymentHandler {
	return &InitiatePaymentHandler{repo: repo}
}

func (h *InitiatePaymentHandler) Handle(ctx context.Context, cmd *dto.InitiatePayment) (*domain.Transaction, error) {
	tx := &domain.Transaction{
		AgentID:     cmd.AgentID,
		CustomerID:  cmd.CustomerID,
		Amount:      cmd.Amount,
		Currency:    cmd.Currency,
		Status:      domain.StatePending,
		CallbackURL: cmd.CallbackURL,
	}

	if err := h.repo.Save(ctx, nil, tx); err != nil {
		return nil, err
	}

	return tx, nil
}
