package port

import (
	"context"

	"github/master_go_cqrs/services/user/internal/domain"
)

// UserRepository provides access to the user_identities store.
// All methods accept a plain context.Context; transactional writes should be
// wrapped in a UnitOfWork, and the adapter extracts the underlying DB
// transaction from the context internally.
type UserRepository interface {
	// Upsert inserts or updates a user identity, keyed on (identity_provider, identity_provider_user_id).
	Upsert(ctx context.Context, user *domain.UserIdentity) error

	// GetByID returns a user identity by its internal UUID.
	GetByID(ctx context.Context, id string) (*domain.UserIdentity, error)

	// GetByProviderUserID returns a user identity by the external provider's user ID.
	GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*domain.UserIdentity, error)

	// GetByUsername returns a user identity by the provider and username.
	GetByUsername(ctx context.Context, provider, username string) (*domain.UserIdentity, error)

	// ListAll returns a paginated list of user identities.
	ListAll(ctx context.Context, limit, offset int) ([]*domain.UserIdentity, int64, error)
}
