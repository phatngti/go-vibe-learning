package domain

import "time"

// AgentBalance is the aggregate root for settlement.
type AgentBalance struct {
	AgentID       string
	ActiveBalance float64
	CreditLimit   float64
	UpdatedAt     time.Time
}

// CommissionLog represents a ledger entry for commission payments.
type CommissionLog struct {
	ID                string
	AgentID           string
	TransactionID     string
	TransactionAmount float64
	CommissionAmount  float64
	Rank              string
	CreatedAt         time.Time
}
