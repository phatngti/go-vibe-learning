package tokenauth

const (
	// Internal code 1000-9999
	// Authenticate 1000-1500
	InvalidToken      = "invalid_token"
	BearerIssued      = "bearer_issued"
	TokenNotFound     = "token_not_found"
	TokenClaimInvalid = "token_claim_invalid"
	TokenAuthenFailed = "token_authen_failed"
	AuthorizeFailed   = "authorize_failed"
	SessionExpired    = "session_expired"
)

var MessageCode = map[string]string{
	InvalidToken:      "Invalid token",
	TokenNotFound:     "Token not found",
	TokenClaimInvalid: "Token claim invalid",
	BearerIssued:      "Bearer issued",
	TokenAuthenFailed: "Token authen failed",
	AuthorizeFailed:   "Authorize failed. Access denied",
	SessionExpired:    "Session expired, please login again",
}
