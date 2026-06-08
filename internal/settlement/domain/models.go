package domain

import (
	"time"
)

type AgentBalance struct {
	AgentID       string    `gorm:"type:text;primaryKey"`
	ActiveBalance float64   `gorm:"type:numeric(15,2);not null;default:0"`
	CreditLimit   float64   `gorm:"type:numeric(15,2);not null;default:1000"` // Allow negative balance up to credit limit
	UpdatedAt     time.Time `gorm:"not null;default:now()"`
}

func (AgentBalance) TableName() string {
	return "public.agent_balances"
}

type CommissionLog struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID           string    `gorm:"type:text;not null"`
	TransactionID     string    `gorm:"type:text;not null"`
	TransactionAmount float64   `gorm:"type:numeric(15,2);not null"`
	CommissionAmount  float64   `gorm:"type:numeric(15,2);not null"`
	Rank              string    `gorm:"type:text;not null"`
	CreatedAt         time.Time `gorm:"not null;default:now()"`
}

func (CommissionLog) TableName() string {
	return "public.commission_ledgers"
}
