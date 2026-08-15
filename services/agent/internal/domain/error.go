package domain

import "errors"

var (
	ErrAgentNotFound              = errors.New("agent not found")
	ErrInvalidAPIKey              = errors.New("invalid API key")
	ErrCallbackFailed             = errors.New("callback delivery failed")
	ErrTenantOwnership            = errors.New("agent configuration belongs to another tenant")
	ErrInvalidCallbackURL         = errors.New("callback URL is not allowed")
	ErrCallbackNotVerified        = errors.New("callback endpoint is not verified")
	ErrCallbackSigningUnavailable = errors.New("callback signing key is not configured")
	ErrCallbackChallengeMissing   = errors.New("callback ownership challenge is not configured")
	ErrCallbackChallengeExpired   = errors.New("callback ownership challenge has expired")
	ErrCallbackChallengeInvalid   = errors.New("callback ownership challenge proof is invalid")
	ErrCallbackChallengeReplayed  = errors.New("callback ownership challenge was already used")
	ErrCallbackDeliveryNotFound   = errors.New("callback delivery was not found")
	ErrCallbackDeliveryNotDead    = errors.New("callback delivery is not in terminal DLQ state")
	ErrCallbackDeliveryLeaseLost  = errors.New("callback delivery processing lease was lost")
	ErrCallbackReplayForbidden    = errors.New("callback replay requires a platform operator")
)
