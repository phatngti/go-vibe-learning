package command

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/domain"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// PaymentReconciler resolves ambiguous and abandoned provider attempts by
// querying the provider with the original immutable idempotency identity. It
// never issues a new charge.
type PaymentReconciler struct {
	uow        port.UnitOfWork
	repo       port.TransactionRepository
	gateway    port.PaymentGateway
	eventStore eventstore.Repository
	logger     logger.Factory
	enabled    bool
	interval   time.Duration
	staleAfter time.Duration
	batchSize  int
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewPaymentReconciler(
	uow port.UnitOfWork,
	repo port.TransactionRepository,
	gateway port.PaymentGateway,
	eventStore eventstore.Repository,
	cfg *config.Config,
	l logger.Factory,
) *PaymentReconciler {
	settings := cfg.Reconciliation
	if settings.Interval <= 0 {
		settings.Interval = 15 * time.Second
	}
	if settings.StaleAfter <= 0 {
		settings.StaleAfter = 2 * time.Minute
	}
	if settings.BatchSize <= 0 {
		settings.BatchSize = 100
	}
	return &PaymentReconciler{
		uow: uow, repo: repo, gateway: gateway, eventStore: eventStore, logger: l,
		enabled: settings.Enabled, interval: settings.Interval, staleAfter: settings.StaleAfter, batchSize: settings.BatchSize,
	}
}

func (r *PaymentReconciler) Start(lifecycle fx.Lifecycle) {
	if !r.enabled {
		return
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			r.cancel = cancel
			r.wg.Add(1)
			go r.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			if r.cancel != nil {
				r.cancel()
			}
			r.wg.Wait()
			return nil
		},
	})
}

func (r *PaymentReconciler) run(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.Reconcile(ctx); err != nil && ctx.Err() == nil {
			r.logger.Bg().Error("Payment reconciliation batch failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *PaymentReconciler) Reconcile(ctx context.Context) error {
	transactions, err := r.repo.ListForReconciliation(ctx, time.Now().Add(-r.staleAfter), r.batchSize)
	if err != nil {
		return err
	}
	for _, transaction := range transactions {
		if err := r.reconcileOne(ctx, transaction); err != nil {
			r.logger.Bg().Warn("Payment reconciliation item failed",
				zap.String("transaction_id", transaction.ID), zap.Error(err))
		}
	}
	return nil
}

func (r *PaymentReconciler) reconcileOne(ctx context.Context, transaction *domain.Transaction) error {
	if transaction.IdempotencyKey == "" {
		return fmt.Errorf("payment %s has no provider idempotency key", transaction.ID)
	}
	lookup, err := r.gateway.LookupCharge(ctx, transaction.IdempotencyKey, transaction.ProviderReference)
	if err != nil {
		return fmt.Errorf("lookup provider charge: %w", err)
	}
	if lookup == nil || !lookup.Found {
		return nil
	}
	state := domain.StateFailed
	reason := lookup.Message
	if lookup.Completed {
		state = domain.StateCompleted
		reason = ""
	} else if reason == "" {
		reason = "payment provider confirmed the charge was not completed"
	}
	return r.uow.Do(ctx, func(txCtx context.Context) error {
		resolved, err := r.repo.ResolveReconciliation(txCtx, transaction.ID, transaction.IdempotencyKey, state, lookup.Reference, reason)
		if err != nil || !resolved {
			return err
		}
		resolvedTransaction := *transaction
		resolvedTransaction.Status = state
		resolvedTransaction.ProviderReference = lookup.Reference
		resolvedTransaction.LastError = reason
		eventType, payload, err := paymentResultEvent(&resolvedTransaction, state, reason)
		if err != nil {
			return err
		}
		// Validate the event representation here so a corrupt provider adapter
		// cannot commit a terminal state without its matching domain fact.
		if !json.Valid(payload) {
			return fmt.Errorf("reconciled payment event is not valid JSON")
		}
		version, err := r.eventStore.NextVersion(txCtx, nil, "Transaction", transaction.ID)
		if err != nil {
			return err
		}
		return r.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
			AggregateType: "Transaction", AggregateID: transaction.ID,
			EventType: eventType, Payload: string(payload), Version: version,
		})
	})
}
