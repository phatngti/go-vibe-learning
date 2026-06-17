package domain

import "context"

// SyncGateway abstracts the external identity provider's admin API for syncing
// user data into the local user_identities table.
type SyncGateway interface {
	// FetchAllUsers retrieves all users from the identity provider.
	FetchAllUsers(ctx context.Context) ([]UserIdentity, error)

	// FetchUserByID retrieves a single user by the identity provider's user ID.
	FetchUserByID(ctx context.Context, providerUserID string) (*UserIdentity, error)
}
