package domain

import "errors"

var (
	ErrAgentNotFound  = errors.New("agent not found")
	ErrInvalidAPIKey  = errors.New("invalid API key")
	ErrCallbackFailed = errors.New("callback delivery failed")
)
