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

// ProcessPaymentHandler charges a payment gateway, transitions the transaction
// state, and atomically appends the resulting domain event to the event store.
// The event store relay automatically forwards matching events to the outbox.
type ProcessPaymentHandler struct {
	db         *gorm.DB
	repo       domain.TransactionRepository
	gateway    domain.PaymentGateway
	eventStore eventstore.Repository
}

func NewProcessPaymentHandler(
	db *platformpostgres.DB,
	repo domain.TransactionRepository,
	gateway domain.PaymentGateway,
	eventStore eventstore.Repository,
) *ProcessPaymentHandler {
	return &ProcessPaymentHandler{
		db:         db.DB,
		repo:       repo,
		gateway:    gateway,
		eventStore: eventStore,
	}
}

func (h *ProcessPaymentHandler) Handle(ctx context.Context, cmd *dto.ProcessPayment) (*domain.Transaction, error) {
	txRecord, err := h.repo.GetByID(ctx, cmd.TransactionID)
	if err != nil {
		return nil, err
	}

	if txRecord.Status != domain.StatePending {
		return nil, fmt.Errorf("transaction %s is already processed", cmd.TransactionID)
	}

	chargeReq := &domain.GatewayChargeRequest{
		TransactionID: txRecord.ID,
		Amount:        txRecord.Amount,
		Currency:      txRecord.Currency,
		CardNumber:    cmd.CardNumber,
		CVV:           cmd.CVV,
	}

	chargeRes, chargeErr := h.gateway.Charge(ctx, chargeReq)
	var finalState domain.TransactionState
	if chargeErr != nil || !chargeRes.Success {
		finalState = domain.StateFailed
	} else {
		finalState = domain.StateCompleted
	}

	var updatedTx *domain.Transaction

	err = h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.UpdateStatus(ctx, dbTx, txRecord.ID, finalState); err != nil {
			return err
		}

		var err error
		updatedTx, err = h.repo.GetByIDWithDB(ctx, dbTx, txRecord.ID)
		if err != nil {
			return err
		}

		now := time.Now()
		var (
			evtType  string
			evtBytes []byte
		)

		if finalState == domain.StateCompleted {
			evt := domain.PaymentCompleted{
				TransactionID: updatedTx.ID,
				AgentID:       updatedTx.AgentID,
				CustomerID:    updatedTx.CustomerID,
				Amount:        updatedTx.Amount,
				Currency:      updatedTx.Currency,
				OccurredAt:    now,
			}
			evtType = evt.EventType()
			evtBytes, err = json.Marshal(evt)
			if err != nil {
				return fmt.Errorf("process_payment: marshal PaymentCompleted: %w", err)
			}
		} else {
			reason := "charge declined"
			if chargeErr != nil {
				reason = chargeErr.Error()
			}
			evt := domain.PaymentFailed{
				TransactionID: updatedTx.ID,
				AgentID:       updatedTx.AgentID,
				Amount:        updatedTx.Amount,
				Currency:      updatedTx.Currency,
				Reason:        reason,
				OccurredAt:    now,
			}
			evtType = evt.EventType()
			evtBytes, err = json.Marshal(evt)
			if err != nil {
				return fmt.Errorf("process_payment: marshal PaymentFailed: %w", err)
			}
		}

		// Append domain event to the event store (regardless of success/failure).
		// The relay automatically enqueues PaymentCompleted to the outbox;
		// PaymentFailed has no relay mapping and stays in the event store only.
		return h.appendEvent(ctx, dbTx, updatedTx.ID, evtType, evtBytes)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to complete process payment transaction: %w", err)
	}

	return updatedTx, nil
}

func (h *ProcessPaymentHandler) appendEvent(ctx context.Context, tx *gorm.DB, aggregateID, eventType string, payload []byte) error {
	version, err := h.eventStore.NextVersion(ctx, tx, "Transaction", aggregateID)
	if err != nil {
		return err
	}
	return h.eventStore.AppendEvent(ctx, tx, &eventstore.DomainEvent{
		AggregateType: "Transaction",
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       string(payload),
		Version:       version,
	})
}
