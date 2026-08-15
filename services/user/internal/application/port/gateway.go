package port

import (
	"context"

	"github/master_go_cqrs/services/user/internal/domain"
)

// SyncGateway abstracts the external identity provider's admin API for syncing
// user data into the local user_identities table.
type SyncGateway interface {
	// FetchAllUsers retrieves all users from the identity provider.
	// It is retained for compatibility; durable workers should use
	// FetchUsersPage so progress can be checkpointed between provider calls.
	FetchAllUsers(ctx context.Context) ([]domain.UserIdentity, error)

	// FetchUsersPage retrieves one stable, bounded provider page. Cursor is the
	// zero-based provider offset and NextCursor is the offset for the next page.
	FetchUsersPage(ctx context.Context, cursor, limit int) (UserSyncPage, error)

	// FetchUserByID retrieves a single user by the identity provider's user ID.
	FetchUserByID(ctx context.Context, providerUserID string) (*domain.UserIdentity, error)
}

// UserSyncPage is the provider-neutral pagination contract used by durable
// synchronization jobs.
type UserSyncPage struct {
	Users      []domain.UserIdentity
	NextCursor int
	Done       bool
}
