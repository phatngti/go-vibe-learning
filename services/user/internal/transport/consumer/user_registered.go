package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/observability/logger"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/user/internal/application/command"
	"github/master_go_cqrs/services/user/internal/domain"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const userRegisteredConsumerName = "user-registered-sync"

var userRegisteredEventTypes = []string{
	(domain.UserRegistered{}).EventType(),
}

type batchConsumer interface {
	ConsumeBatch(context.Context, string, string, []string, int, eventstore.BatchHandler) (int, error)
}

// UserRegisteredHandler translates UserRegistered events into durable sync-job
// intent. It performs no external I/O while the event consumer holds its
// checkpoint transaction.
type UserRegisteredHandler struct {
	requestSync *command.RequestUserSyncHandler
	logger      logger.Factory
}

func NewUserRegisteredHandler(
	requestSync *command.RequestUserSyncHandler,
	logger logger.Factory,
) *UserRegisteredHandler {
	return &UserRegisteredHandler{requestSync: requestSync, logger: logger}
}

func (h *UserRegisteredHandler) Handle(ctx context.Context, tx *gorm.DB, event *eventstore.DomainEvent) error {
	var evt domain.UserRegistered
	if err := json.Unmarshal([]byte(event.Payload), &evt); err != nil {
		return fmt.Errorf("decode UserRegistered event: %w", err)
	}

	h.logger.For(ctx).Info("Processing UserRegistered event").WithFields(
		zap.String("event_id", event.ID.String()),
		zap.String("provider_user_id", evt.IdentityProviderUserID),
	)

	// The job insert uses the event consumer's transaction, making durable
	// intent and checkpoint advancement atomic. A separate worker performs the
	// Keycloak call only after this transaction commits.
	txCtx := platformpostgres.WithTx(ctx, tx)
	job, created, err := h.requestSync.EnqueueRegistered(
		txCtx, event.ID.String(), evt.IdentityProviderUserID,
	)
	if err != nil {
		return err
	}

	h.logger.For(ctx).Info("User synchronization job persisted").WithFields(
		zap.String("sync_job_id", job.ID),
		zap.String("provider_user_id", evt.IdentityProviderUserID),
		zap.Bool("created", created),
	)
	return nil
}

// UserRegisteredWorker is a background worker that polls the event store for
// UserRegistered events and processes them. Failed events are automatically
// retried on the next poll cycle.
type UserRegisteredWorker struct {
	consumer batchConsumer
	handler  *UserRegisteredHandler
	logger   logger.Factory
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewUserRegisteredWorker(
	consumer *eventstore.Consumer,
	handler *UserRegisteredHandler,
	logger logger.Factory,
) *UserRegisteredWorker {
	return &UserRegisteredWorker{consumer: consumer, handler: handler, logger: logger}
}

func (w *UserRegisteredWorker) Start(lifecycle fx.Lifecycle) {
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

func (w *UserRegisteredWorker) run(ctx context.Context) {
	defer w.wg.Done()
	for {
		processed, err := w.consumer.ConsumeBatch(
			ctx, userRegisteredConsumerName, aggregateType,
			userRegisteredEventTypes, 100, w.handler.Handle,
		)
		if err != nil && ctx.Err() == nil {
			w.logger.Bg().Error("UserRegistered consumer failed").WithFields(zap.Error(err))
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

const aggregateType = "UserIdentity"
