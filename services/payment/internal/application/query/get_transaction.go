package query

import (
	"context"

	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type GetTransactionHandler struct {
	repo port.TransactionRepository
}

func NewGetTransactionHandler(repo port.TransactionRepository) *GetTransactionHandler {
	return &GetTransactionHandler{repo: repo}
}

func (h *GetTransactionHandler) Handle(ctx context.Context, id string) (*domain.Transaction, error) {
	return h.repo.GetByID(ctx, id)
}
