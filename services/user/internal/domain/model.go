package domain

import "time"

// UserIdentity is the aggregate root representing a synced user from an
// external identity provider (e.g. Keycloak).
type UserIdentity struct {
	ID                     string
	IdentityProvider       string
	IdentityProviderUserID string
	Username               string
	Email                  *string
	FirstName              *string
	LastName               *string
	EmailVerified          bool
	Enabled                bool
	Attributes             string
	IdentityCreatedAt      *time.Time
	LastSyncedAt           time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// RegisterCredentials holds the information needed to register a new user
// with an external identity provider. This is a domain value object that
// replaces the previous coupling to application DTOs.
type RegisterCredentials struct {
	Email    string
	Password string
}

// TokenPair holds the access and refresh tokens returned by the auth provider.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}
