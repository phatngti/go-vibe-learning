package domain

import (
	"time"
)

type AgentConfig struct {
	AgentID     string    `gorm:"type:text;primaryKey"`
	CallbackURL string    `gorm:"type:text;not null"`
	ApiKey      string    `gorm:"type:text;uniqueIndex;not null"`
	AllowedIPs  string    `gorm:"type:text;default:''"` // Comma-separated whitelisted IPs
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (AgentConfig) TableName() string {
	return "public.agent_configs"
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AgentID   string    `gorm:"type:text;not null"`
	Action    string    `gorm:"type:text;not null"`
	Details   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (AuditLog) TableName() string {
	return "public.agent_audit_logs"
}

type CallbackDelivery struct {
	TransactionID string    `gorm:"type:text;primaryKey"`
	AgentID       string    `gorm:"type:text;not null"`
	CallbackURL   string    `gorm:"type:text"`
	Status        string    `gorm:"type:text;not null"`
	Attempts      int       `gorm:"not null;default:0"`
	LastError     string    `gorm:"type:text"`
	UpdatedAt     time.Time `gorm:"not null;default:now()"`
}

func (CallbackDelivery) TableName() string {
	return "public.callback_deliveries"
}
