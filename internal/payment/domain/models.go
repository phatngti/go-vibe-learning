package domain

import (
	"time"
)

type TransactionState string

const (
	StatePending    TransactionState = "PENDING"
	StateProcessing TransactionState = "PROCESSING"
	StateCompleted  TransactionState = "COMPLETED"
	StateFailed     TransactionState = "FAILED"
)

type Transaction struct {
	ID          string           `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID     string           `gorm:"type:text;not null"`
	CustomerID  string           `gorm:"type:text;not null"`
	Amount      float64          `gorm:"type:numeric(15,2);not null"`
	Currency    string           `gorm:"type:text;not null"`
	Status      TransactionState `gorm:"type:text;not null;default:'PENDING'"`
	CallbackURL string           `gorm:"type:text"`
	CreatedAt   time.Time        `gorm:"not null;default:now()"`
	UpdatedAt   time.Time        `gorm:"not null;default:now()"`
}

func (Transaction) TableName() string {
	return "public.transactions"
}
