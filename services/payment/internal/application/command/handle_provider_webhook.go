package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type ProviderWebhookResult struct {
	EventID       string
	Duplicate     bool
	Ignored       bool
	TransactionID string
}

type HandleProviderWebhookHandler struct {
	uow        port.UnitOfWork
	repo       port.PaymentV2Repository
	provider   port.PaymentProvider
	eventStore eventstore.Repository
}

func NewHandleProviderWebhookHandler(
	uow port.UnitOfWork,
	repo port.PaymentV2Repository,
	provider port.PaymentProvider,
	eventStore eventstore.Repository,
) *HandleProviderWebhookHandler {
	return &HandleProviderWebhookHandler{uow: uow, repo: repo, provider: provider, eventStore: eventStore}
}

func (h *HandleProviderWebhookHandler) Handle(ctx context.Context, payload []byte, signature string, receivedAt time.Time) (*ProviderWebhookResult, error) {
	event, err := h.provider.ParseWebhook(payload, signature, receivedAt)
	if err != nil {
		return nil, fmt.Errorf("parse %s webhook: %w", h.provider.Name(), err)
	}
	if event == nil || event.EventID == "" {
		return nil, fmt.Errorf("parse %s webhook: provider returned an invalid event", h.provider.Name())
	}
	result := &ProviderWebhookResult{EventID: event.EventID, TransactionID: event.TransactionID}
	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		created, recordErr := h.repo.TryRecordProviderEvent(txCtx, *event)
		if recordErr != nil {
			return recordErr
		}
		if !created {
			result.Duplicate = true
			return nil
		}

		targetState, terminal := providerTerminalState(event.Status)
		if strings.TrimSpace(event.TransactionID) == "" {
			result.Ignored = true
			return h.repo.MarkProviderEventProcessed(txCtx, event.Provider, event.EventID, "IGNORED", "missing internal transaction metadata")
		}
		var target *domain.TransactionState
		if terminal {
			target = &targetState
		}
		tx, changed, applyErr := h.repo.ApplyProviderOutcome(
			txCtx, event.TransactionID, event.Provider, event.ProviderReference, string(event.Status), target, event.FailureReason,
		)
		if applyErr != nil {
			return applyErr
		}
		if changed {
			reason := event.FailureReason
			if targetState == domain.StateFailed && reason == "" {
				reason = "payment provider reported a terminal failure"
			}
			eventType, eventPayload, mapErr := paymentResultEvent(tx, targetState, reason)
			if mapErr != nil {
				return mapErr
			}
			version, versionErr := h.eventStore.NextVersion(txCtx, nil, "Transaction", tx.ID)
			if versionErr != nil {
				return versionErr
			}
			if appendErr := h.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
				AggregateType: "Transaction", AggregateID: tx.ID, EventType: eventType,
				Payload: string(eventPayload), Version: version,
			}); appendErr != nil {
				return appendErr
			}
		}
		return h.repo.MarkProviderEventProcessed(txCtx, event.Provider, event.EventID, "PROCESSED", "")
	})
	if err != nil {
		return nil, fmt.Errorf("handle provider webhook %s: %w", event.EventID, err)
	}
	return result, nil
}

func providerTerminalState(status port.ProviderPaymentStatus) (domain.TransactionState, bool) {
	switch status {
	case port.ProviderStatusSucceeded:
		return domain.StateCompleted, true
	case port.ProviderStatusCanceled, port.ProviderStatusFailed:
		return domain.StateFailed, true
	default:
		return "", false
	}
}
