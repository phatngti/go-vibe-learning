package dto

var MessageCode = map[string]string{}

type ConfigureCallback struct {
	AgentID     string   `json:"agent_id" example:"agent-123"`
	TenantID    string   `json:"-"`
	CallbackURL string   `json:"callback_url" example:"https://merchant.example/callback"`
	AllowedIPs  []string `json:"allowed_ips"`
}

type VerifyCallback struct {
	AgentID        string `json:"agent_id" example:"agent-123"`
	TenantID       string `json:"-"`
	ChallengeID    string `json:"challenge_id" validate:"required"`
	ChallengeToken string `json:"challenge_token" validate:"required"`
}

type GenerateAPIKey struct {
	AgentID  string `json:"agent_id" example:"agent-123"`
	TenantID string `json:"-"`
}

type ReplayCallback struct {
	EventID          string `param:"event_id" json:"-"`
	Reason           string `json:"reason" validate:"required"`
	TenantID         string `json:"-"`
	OperatorID       string `json:"-"`
	PlatformOperator bool   `json:"-"`
}

type RecordAuditLog struct {
	AgentID string `json:"agent_id" example:"agent-123"`
	Action  string `json:"action" example:"generate_api_key"`
	Details string `json:"details" example:"API key generated"`
}

type AgentConfigResponse struct {
	AgentID            string `json:"agent_id" example:"agent-123"`
	CallbackURL        string `json:"callback_url" example:"https://merchant.example/callback"`
	CallbackStatus     string `json:"callback_status" example:"PENDING_VERIFICATION"`
	ChallengeID        string `json:"challenge_id,omitempty"`
	ChallengeToken     string `json:"challenge_token,omitempty"`
	ChallengeProofURL  string `json:"challenge_proof_url,omitempty"`
	ChallengeExpiresAt string `json:"challenge_expires_at,omitempty"`
	APIKey             string `json:"api_key,omitempty" example:"generated-key"`
	AllowedIPs         string `json:"allowed_ips" example:"127.0.0.1,10.0.0.1"`
	UpdatedAt          string `json:"updated_at" example:"2026-06-06T12:00:00+07:00"`
}

type VerifyCallbackResponse struct {
	AgentID          string `json:"agent_id"`
	CallbackURL      string `json:"callback_url"`
	CallbackStatus   string `json:"callback_status"`
	CallbackVerified string `json:"callback_verified_at"`
}

type GenerateAPIKeyResponse struct {
	AgentID string `json:"agent_id" example:"agent-123"`
	APIKey  string `json:"api_key" example:"generated-key"`
}

type ReplayCallbackResponse struct {
	EventID           string `json:"event_id"`
	Status            string `json:"status"`
	OutcomeGeneration int64  `json:"outcome_generation"`
	ReplayCount       int    `json:"replay_count"`
}
