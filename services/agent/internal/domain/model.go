package domain

import "time"

// AgentConfig holds the configuration for an API agent.
type AgentConfig struct {
	AgentID     string
	CallbackURL string
	ApiKey      string
	AllowedIPs  string
	UpdatedAt   time.Time
}

// AuditLog records agent actions for compliance.
type AuditLog struct {
	ID        uint
	AgentID   string
	Action    string
	Details   string
	CreatedAt time.Time
}

// CallbackDelivery tracks webhook delivery status.
type CallbackDelivery struct {
	TransactionID string
	AgentID       string
	CallbackURL   string
	Status        string
	Attempts      int
	LastError     string
	UpdatedAt     time.Time
}
