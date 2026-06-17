package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/domain"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

// InitiatePaymentHandler creates a new PENDING transaction and appends a
// PaymentInitiated domain event to the event store within the same DB transaction.
type InitiatePaymentHandler struct {
	db         *gorm.DB
	repo       domain.TransactionRepository
	eventStore eventstore.Repository
}

func NewInitiatePaymentHandler(db *platformpostgres.DB, repo domain.TransactionRepository, eventStore eventstore.Repository) *InitiatePaymentHandler {
	return &InitiatePaymentHandler{
		db:         db.DB,
		repo:       repo,
		eventStore: eventStore,
	}
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

	var savedTx *domain.Transaction

	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.Save(ctx, dbTx, tx); err != nil {
			return err
		}
		savedTx = tx

		evt := domain.PaymentInitiated{
			TransactionID: tx.ID,
			AgentID:       tx.AgentID,
			CustomerID:    tx.CustomerID,
			Amount:        tx.Amount,
			Currency:      tx.Currency,
			CallbackURL:   tx.CallbackURL,
			OccurredAt:    time.Now(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("initiate_payment: marshal event: %w", err)
		}
		version, err := h.eventStore.NextVersion(ctx, dbTx, "Transaction", tx.ID)
		if err != nil {
			return err
		}
		return h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
			AggregateType: "Transaction",
			AggregateID:   tx.ID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("initiate_payment: %w", err)
	}

	return savedTx, nil
}
