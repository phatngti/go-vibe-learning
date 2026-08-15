package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

const processingLease = 30 * time.Second

// ProcessPaymentHandler claims one logical payment attempt before contacting
// the provider. Provider calls use a stable idempotency key, while the final
// transaction state, domain event, and relay outbox row commit atomically.
type ProcessPaymentHandler struct {
	uow        port.UnitOfWork
	repo       port.TransactionRepository
	gateway    port.PaymentGateway
	eventStore eventstore.Repository
}

func NewProcessPaymentHandler(
	uow port.UnitOfWork,
	repo port.TransactionRepository,
	gateway port.PaymentGateway,
	eventStore eventstore.Repository,
) *ProcessPaymentHandler {
	return &ProcessPaymentHandler{uow: uow, repo: repo, gateway: gateway, eventStore: eventStore}
}

func (h *ProcessPaymentHandler) Handle(ctx context.Context, cmd *dto.ProcessPayment) (*domain.Transaction, error) {
	idempotencyKey := cmd.IdempotencyKey
	if idempotencyKey == "" {
		// A payment transaction represents one provider charge attempt. The stable
		// fallback makes retries safe even for legacy callers that do not yet send
		// an explicit key.
		idempotencyKey = "payment:" + cmd.TransactionID + ":charge"
	}

	txRecord, claimed, err := h.repo.ClaimForProcessing(ctx, cmd.TransactionID, idempotencyKey, time.Now().Add(-processingLease))
	if err != nil {
		return nil, err
	}
	if !claimed {
		switch txRecord.Status {
		case domain.StateCompleted, domain.StateFailed:
			return txRecord, nil
		default:
			return nil, domain.ErrPaymentInProgress
		}
	}

	chargeRes, chargeErr := h.gateway.Charge(ctx, &domain.GatewayChargeRequest{
		TransactionID:  txRecord.ID,
		Amount:         txRecord.Amount,
		Currency:       txRecord.Currency,
		CardNumber:     cmd.CardNumber,
		CVV:            cmd.CVV,
		IdempotencyKey: idempotencyKey,
	})

	finalState := domain.StateUnknown
	lastError := ""
	providerReference := ""
	if chargeErr != nil {
		// Transport/system errors are ambiguous. The provider may have accepted
		// the charge before the response was lost.
		lastError = chargeErr.Error()
	} else if chargeRes == nil {
		lastError = "payment provider returned an empty response"
	} else {
		providerReference = chargeRes.Reference
		if chargeRes.Success {
			finalState = domain.StateCompleted
		} else {
			finalState = domain.StateFailed
			lastError = chargeRes.Message
			if lastError == "" {
				lastError = "payment provider declined the charge"
			}
		}
	}

	var updatedTx *domain.Transaction
	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		if err := h.repo.FinalizeProcessing(txCtx, txRecord.ID, idempotencyKey, finalState, providerReference, lastError); err != nil {
			return err
		}

		var err error
		updatedTx, err = h.repo.GetByID(txCtx, txRecord.ID)
		if err != nil {
			return err
		}
		eventType, payload, err := paymentResultEvent(updatedTx, finalState, lastError)
		if err != nil {
			return err
		}
		version, err := h.eventStore.NextVersion(txCtx, nil, "Transaction", updatedTx.ID)
		if err != nil {
			return err
		}
		return h.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
			AggregateType: "Transaction",
			AggregateID:   updatedTx.ID,
			EventType:     eventType,
			Payload:       string(payload),
			Version:       version,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("finalize payment result: %w", err)
	}
	return updatedTx, nil
}

func paymentResultEvent(tx *domain.Transaction, state domain.TransactionState, reason string) (string, []byte, error) {
	now := time.Now()
	monetaryAmount, err := tx.MonetaryAmount()
	if err != nil {
		return "", nil, fmt.Errorf("resolve payment money: %w", err)
	}
	var event domain.Event
	switch state {
	case domain.StateCompleted:
		event = domain.PaymentCompleted{
			TransactionID: tx.ID, AgentID: tx.AgentID, CustomerID: tx.CustomerID,
			Money: &monetaryAmount, Amount: tx.Amount, Currency: tx.Currency, OccurredAt: now,
		}
	case domain.StateFailed:
		event = domain.PaymentFailed{
			TransactionID: tx.ID, AgentID: tx.AgentID, Money: &monetaryAmount, Amount: tx.Amount,
			Currency: tx.Currency, Reason: reason, OccurredAt: now,
		}
	case domain.StateUnknown:
		event = domain.PaymentProcessingUncertain{
			TransactionID: tx.ID, AgentID: tx.AgentID, Money: &monetaryAmount, Amount: tx.Amount,
			Currency: tx.Currency, Reason: reason, OccurredAt: now,
		}
	default:
		return "", nil, fmt.Errorf("unsupported payment result state %q", state)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return "", nil, fmt.Errorf("marshal %s: %w", event.EventType(), err)
	}
	return event.EventType(), payload, nil
}
