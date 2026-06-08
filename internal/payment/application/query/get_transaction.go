package query

import (
	"context"

	"github/master_go_cqrs/internal/payment/domain"
)

type GetTransactionHandler struct {
	repo domain.TransactionRepository
}

func NewGetTransactionHandler(repo domain.TransactionRepository) *GetTransactionHandler {
	return &GetTransactionHandler{repo: repo}
}

func (h *GetTransactionHandler) Handle(ctx context.Context, id string) (*domain.Transaction, error) {
	return h.repo.GetByID(ctx, id)
}
