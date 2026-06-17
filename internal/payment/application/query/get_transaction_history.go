package query

import (
	"context"

	"github/master_go_cqrs/internal/platform/eventstore"
)

// GetTransactionHistoryHandler returns the full append-only event log for a
// transaction aggregate — providing a complete audit trail.
type GetTransactionHistoryHandler struct {
	eventStore eventstore.Repository
}

func NewGetTransactionHistoryHandler(es eventstore.Repository) *GetTransactionHistoryHandler {
	return &GetTransactionHistoryHandler{eventStore: es}
}

func (h *GetTransactionHistoryHandler) Handle(ctx context.Context, transactionID string) ([]*eventstore.DomainEvent, error) {
	return h.eventStore.LoadEvents(ctx, "Transaction", transactionID)
}
