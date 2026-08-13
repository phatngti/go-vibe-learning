package domain

import "errors"

var (
	// ErrUserNotFound is returned when a user cannot be found.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidCredentials is returned when login credentials are invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrRegistrationFailed is returned when user registration fails.
	ErrRegistrationFailed = errors.New("registration failed")

	// ErrUserAlreadyExists is returned when a user with the same identity already exists.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrTokenRefreshFailed is returned when token refresh fails.
	ErrTokenRefreshFailed = errors.New("token refresh failed")
)
