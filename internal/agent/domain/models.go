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
