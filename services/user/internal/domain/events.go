package domain

import "time"

// Event represents a domain event that occurred within the User aggregate.
type Event interface {
	EventType() string
}

// UserSynced is raised when a user identity is created or updated from
// the external identity provider during a sync operation.
type UserSynced struct {
	UserID                 string    `json:"user_id"`
	IdentityProvider       string    `json:"identity_provider"`
	IdentityProviderUserID string    `json:"identity_provider_user_id"`
	Username               string    `json:"username"`
	Email                  string    `json:"email,omitempty"`
	FirstName              string    `json:"first_name,omitempty"`
	LastName               string    `json:"last_name,omitempty"`
	EmailVerified          bool      `json:"email_verified"`
	Enabled                bool      `json:"enabled"`
	OccurredAt             time.Time `json:"occurred_at"`
}

func (UserSynced) EventType() string { return "UserSynced" }

// UserRegistered is raised when a user completes registration via the identity
// provider. It triggers an asynchronous sync of the user into the local database.
type UserRegistered struct {
	IdentityProviderUserID string    `json:"identity_provider_user_id"`
	Email                  string    `json:"email"`
	OccurredAt             time.Time `json:"occurred_at"`
}

func (UserRegistered) EventType() string { return "UserRegistered" }

// UserDisabled is raised when a user is disabled in the identity provider.
type UserDisabled struct {
	UserID                 string    `json:"user_id"`
	IdentityProviderUserID string    `json:"identity_provider_user_id"`
	Reason                 string    `json:"reason,omitempty"`
	OccurredAt             time.Time `json:"occurred_at"`
}

func (UserDisabled) EventType() string { return "UserDisabled" }
