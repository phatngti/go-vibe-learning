package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"
)

const aggregateType = "UserIdentity"

// SyncUsersHandler orchestrates the full user sync flow:
// 1. Fetch users from the external identity provider.
// 2. Upsert each user into the local user_identities table.
// 3. Append a UserSynced domain event per upserted user.
type SyncUsersHandler struct {
	uow        port.UnitOfWork
	repo       port.UserRepository
	syncGw     port.SyncGateway
	eventStore eventstore.Repository
}

func NewSyncUsersHandler(
	uow port.UnitOfWork,
	repo port.UserRepository,
	syncGw port.SyncGateway,
	es eventstore.Repository,
) *SyncUsersHandler {
	return &SyncUsersHandler{
		uow:        uow,
		repo:       repo,
		syncGw:     syncGw,
		eventStore: es,
	}
}

func (h *SyncUsersHandler) Handle(ctx context.Context) (int, error) {
	users, err := h.syncGw.FetchAllUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("sync_users: fetch: %w", err)
	}

	synced := 0
	for i := range users {
		if err := h.SyncOne(ctx, &users[i]); err != nil {
			return synced, fmt.Errorf("sync_users: user %s: %w", users[i].IdentityProviderUserID, err)
		}
		synced++
	}
	return synced, nil
}

// SyncOne upserts a single user identity and appends a UserSynced domain event
// within the same database transaction.
func (h *SyncUsersHandler) SyncOne(ctx context.Context, user *domain.UserIdentity) error {
	return h.uow.Do(ctx, func(txCtx context.Context) error {
		return h.syncOne(txCtx, user)
	})
}

// SyncBatch commits one bounded provider page and an optional durable-job
// transition in the same local transaction. The provider call must happen
// before this method is invoked; no external I/O is performed while the
// transaction is open.
func (h *SyncUsersHandler) SyncBatch(
	ctx context.Context,
	users []domain.UserIdentity,
	after func(txCtx context.Context) error,
) error {
	return h.uow.Do(ctx, func(txCtx context.Context) error {
		for i := range users {
			if err := h.syncOne(txCtx, &users[i]); err != nil {
				return fmt.Errorf("sync_users: user %s: %w", users[i].IdentityProviderUserID, err)
			}
		}
		if after != nil {
			return after(txCtx)
		}
		return nil
	})
}

func (h *SyncUsersHandler) syncOne(txCtx context.Context, user *domain.UserIdentity) error {
	if err := h.repo.Upsert(txCtx, user); err != nil {
		return err
	}

	evt := domain.UserSynced{
		UserID:                 user.ID,
		IdentityProvider:       user.IdentityProvider,
		IdentityProviderUserID: user.IdentityProviderUserID,
		Username:               user.Username,
		EmailVerified:          user.EmailVerified,
		Enabled:                user.Enabled,
		OccurredAt:             time.Now(),
	}
	if user.Email != nil {
		evt.Email = *user.Email
	}
	if user.FirstName != nil {
		evt.FirstName = *user.FirstName
	}
	if user.LastName != nil {
		evt.LastName = *user.LastName
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("sync_users: marshal event: %w", err)
	}

	version, err := h.eventStore.NextVersion(txCtx, nil, aggregateType, user.ID)
	if err != nil {
		return err
	}

	return h.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
		AggregateType: aggregateType,
		AggregateID:   user.ID,
		EventType:     evt.EventType(),
		Payload:       string(payload),
		Version:       version,
	})
}
