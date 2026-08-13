package dto

var MessageCode = map[string]string{}

type ConfigureCallback struct {
	AgentID     string   `json:"agent_id" example:"agent-123"`
	CallbackURL string   `json:"callback_url" example:"https://merchant.example/callback"`
	AllowedIPs  []string `json:"allowed_ips"`
}

type GenerateAPIKey struct {
	AgentID string `json:"agent_id" example:"agent-123"`
}

type RecordAuditLog struct {
	AgentID string `json:"agent_id" example:"agent-123"`
	Action  string `json:"action" example:"generate_api_key"`
	Details string `json:"details" example:"API key generated"`
}

type AgentConfigResponse struct {
	AgentID     string `json:"agent_id" example:"agent-123"`
	CallbackURL string `json:"callback_url" example:"https://merchant.example/callback"`
	APIKey      string `json:"api_key" example:"generated-key"`
	AllowedIPs  string `json:"allowed_ips" example:"127.0.0.1,10.0.0.1"`
	UpdatedAt   string `json:"updated_at" example:"2026-06-06T12:00:00+07:00"`
}

type GenerateAPIKeyResponse struct {
	AgentID string `json:"agent_id" example:"agent-123"`
	APIKey  string `json:"api_key" example:"generated-key"`
}
