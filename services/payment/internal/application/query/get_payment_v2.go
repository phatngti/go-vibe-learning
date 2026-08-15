package query

import (
	"context"

	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type GetPaymentV2Handler struct {
	repo port.TransactionRepository
}

func NewGetPaymentV2Handler(repo port.TransactionRepository) *GetPaymentV2Handler {
	return &GetPaymentV2Handler{repo: repo}
}

func (h *GetPaymentV2Handler) Handle(ctx context.Context, transactionID, ownerID string) (*domain.Transaction, error) {
	tx, err := h.repo.GetByID(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	if tx.AgentID != ownerID {
		// Do not reveal the existence of another owner's transaction.
		return nil, domain.ErrTransactionNotFound
	}
	return tx, nil
}
