package port

import (
	"context"

	"github/master_go_cqrs/services/user/internal/domain"
)

// AuthProvider abstracts the identity provider's authentication operations.
// Uses domain value objects instead of application DTOs to maintain clean
// dependency direction (port depends on domain only).
type AuthProvider interface {
	// Register creates a new user in the identity provider and returns a token pair.
	Register(ctx context.Context, credentials domain.RegisterCredentials) (domain.TokenPair, error)

	// Login authenticates a user and returns a token pair.
	Login(ctx context.Context, username, password string) (domain.TokenPair, error)

	// RefreshToken exchanges a refresh token for a new token pair.
	RefreshToken(ctx context.Context, refreshToken string) (domain.TokenPair, error)
}
