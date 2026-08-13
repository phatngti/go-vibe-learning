package port

import (
	"context"

	"github/master_go_cqrs/services/user/internal/domain"
)

// SyncGateway abstracts the external identity provider's admin API for syncing
// user data into the local user_identities table.
type SyncGateway interface {
	// FetchAllUsers retrieves all users from the identity provider.
	FetchAllUsers(ctx context.Context) ([]domain.UserIdentity, error)

	// FetchUserByID retrieves a single user by the identity provider's user ID.
	FetchUserByID(ctx context.Context, providerUserID string) (*domain.UserIdentity, error)
}
