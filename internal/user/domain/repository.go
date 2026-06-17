package domain

import (
	"context"

	"gorm.io/gorm"
)

// UserRepository provides access to the user_identities table.
type UserRepository interface {
	// Upsert inserts or updates a user identity, keyed on (identity_provider, identity_provider_user_id).
	Upsert(ctx context.Context, db *gorm.DB, user *UserIdentity) error

	// GetByID returns a user identity by its internal UUID.
	GetByID(ctx context.Context, id string) (*UserIdentity, error)

	// GetByProviderUserID returns a user identity by the external provider's user ID.
	GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*UserIdentity, error)

	// GetByUsername returns a user identity by the provider and username.
	GetByUsername(ctx context.Context, provider, username string) (*UserIdentity, error)

	// ListAll returns a paginated list of user identities.
	ListAll(ctx context.Context, limit, offset int) ([]*UserIdentity, int64, error)
}
