package postgres

import "time"

type AgentConfigRow struct {
	AgentID     string    `gorm:"type:text;primaryKey"`
	CallbackURL string    `gorm:"type:text;not null"`
	ApiKey      string    `gorm:"type:text;uniqueIndex;not null"`
	AllowedIPs  string    `gorm:"type:text;default:''"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (AgentConfigRow) TableName() string { return "public.agent_configs" }

type AuditLogRow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AgentID   string    `gorm:"type:text;not null"`
	Action    string    `gorm:"type:text;not null"`
	Details   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (AuditLogRow) TableName() string { return "public.agent_audit_logs" }

type CallbackDeliveryRow struct {
	TransactionID string    `gorm:"type:text;primaryKey"`
	AgentID       string    `gorm:"type:text;not null"`
	CallbackURL   string    `gorm:"type:text"`
	Status        string    `gorm:"type:text;not null"`
	Attempts      int       `gorm:"not null;default:0"`
	LastError     string    `gorm:"type:text"`
	UpdatedAt     time.Time `gorm:"not null;default:now()"`
}

func (CallbackDeliveryRow) TableName() string { return "public.callback_deliveries" }
