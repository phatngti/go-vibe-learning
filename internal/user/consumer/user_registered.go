package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/user/application/command"
	"github/master_go_cqrs/internal/user/domain"

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

// UserRegisteredHandler processes UserRegistered domain events by fetching the
// user from the identity provider and syncing them into the local database.
type UserRegisteredHandler struct {
	syncGw   domain.SyncGateway
	syncUser *command.SyncUsersHandler
	logger   logger.Factory
}

func NewUserRegisteredHandler(
	syncGw domain.SyncGateway,
	syncUser *command.SyncUsersHandler,
	logger logger.Factory,
) *UserRegisteredHandler {
	return &UserRegisteredHandler{syncGw: syncGw, syncUser: syncUser, logger: logger}
}

func (h *UserRegisteredHandler) Handle(ctx context.Context, _ *gorm.DB, event *eventstore.DomainEvent) error {
	var evt domain.UserRegistered
	if err := json.Unmarshal([]byte(event.Payload), &evt); err != nil {
		return fmt.Errorf("decode UserRegistered event: %w", err)
	}

	h.logger.For(ctx).Info("Processing UserRegistered event").WithFields(
		zap.String("event_id", event.ID.String()),
		zap.String("provider_user_id", evt.IdentityProviderUserID),
		zap.String("email", evt.Email),
	)

	// Fetch the full user profile from the identity provider.
	user, err := h.syncGw.FetchUserByID(ctx, evt.IdentityProviderUserID)
	if err != nil {
		return fmt.Errorf("fetch user %s from provider: %w", evt.IdentityProviderUserID, err)
	}

	// Upsert the user into the local database and emit a UserSynced event.
	if err := h.syncUser.SyncOne(ctx, user); err != nil {
		return fmt.Errorf("sync user %s to local DB: %w", evt.IdentityProviderUserID, err)
	}

	h.logger.For(ctx).Info("User synced to local database").WithFields(
		zap.String("user_id", user.ID),
		zap.String("provider_user_id", user.IdentityProviderUserID),
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
