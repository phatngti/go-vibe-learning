package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/user/domain"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

const aggregateType = "UserIdentity"

// SyncUsersHandler orchestrates the full user sync flow:
// 1. Fetch users from the external identity provider.
// 2. Upsert each user into the local user_identities table.
// 3. Append a UserSynced domain event per upserted user.
type SyncUsersHandler struct {
	db         *gorm.DB
	repo       domain.UserRepository
	syncGw     domain.SyncGateway
	eventStore eventstore.Repository
}

func NewSyncUsersHandler(
	db *platformpostgres.DB,
	repo domain.UserRepository,
	syncGw domain.SyncGateway,
	es eventstore.Repository,
) *SyncUsersHandler {
	return &SyncUsersHandler{
		db:         db.DB,
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
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := h.repo.Upsert(ctx, tx, user); err != nil {
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

		version, err := h.eventStore.NextVersion(ctx, tx, aggregateType, user.ID)
		if err != nil {
			return err
		}

		return h.eventStore.AppendEvent(ctx, tx, &eventstore.DomainEvent{
			AggregateType: aggregateType,
			AggregateID:   user.ID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		})
	})
}
