package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/payment/domain"
	"github/master_go_cqrs/internal/platform/eventstore"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const paymentEventConsumerName = "payment-domain-events"

var paymentEventTypes = []string{
	(domain.PaymentInitiated{}).EventType(),
	(domain.PaymentCompleted{}).EventType(),
	(domain.PaymentFailed{}).EventType(),
}

type batchConsumer interface {
	ConsumeBatch(context.Context, string, string, []string, int, eventstore.BatchHandler) (int, error)
}

type EventStoreHandler struct {
	logger logger.Factory
}

func NewEventStoreHandler(logger logger.Factory) *EventStoreHandler {
	return &EventStoreHandler{logger: logger}
}

func (h *EventStoreHandler) Handle(ctx context.Context, _ *gorm.DB, event *eventstore.DomainEvent) error {
	var payload any
	switch event.EventType {
	case (domain.PaymentInitiated{}).EventType():
		payload = &domain.PaymentInitiated{}
	case (domain.PaymentCompleted{}).EventType():
		payload = &domain.PaymentCompleted{}
	case (domain.PaymentFailed{}).EventType():
		payload = &domain.PaymentFailed{}
	default:
		return fmt.Errorf("unsupported payment event type %q", event.EventType)
	}
	if err := json.Unmarshal([]byte(event.Payload), payload); err != nil {
		return fmt.Errorf("decode %s: %w", event.EventType, err)
	}
	h.logger.For(ctx).Info("Consumed payment domain event").WithFields(
		zap.String("event_id", event.ID.String()), zap.String("event_type", event.EventType),
		zap.String("transaction_id", event.AggregateID),
	)
	return nil
}

type EventStoreWorker struct {
	consumer batchConsumer
	handler  *EventStoreHandler
	logger   logger.Factory
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewEventStoreWorker(consumer *eventstore.Consumer, handler *EventStoreHandler, logger logger.Factory) *EventStoreWorker {
	return &EventStoreWorker{consumer: consumer, handler: handler, logger: logger}
}

func (w *EventStoreWorker) Start(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			w.cancel = cancel
			w.wg.Add(1)
			go w.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			if w.cancel != nil {
				w.cancel()
			}
			w.wg.Wait()
			return nil
		},
	})
}

func (w *EventStoreWorker) run(ctx context.Context) {
	defer w.wg.Done()
	for {
		processed, err := w.consumer.ConsumeBatch(ctx, paymentEventConsumerName, "Transaction", paymentEventTypes, 100, w.handler.Handle)
		if err != nil && ctx.Err() == nil {
			w.logger.Bg().Error("Payment event-store consumer failed").WithFields(zap.Error(err))
		}
		delay := 500 * time.Millisecond
		if err != nil {
			delay = time.Second
		} else if processed == 100 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
