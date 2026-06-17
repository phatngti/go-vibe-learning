package model

import (
	"time"

	"github/master_go_cqrs/internal/orchestrator/features/payment"
)

type Model struct {
	TransactionID string        `gorm:"type:text;primaryKey"`
	AgentID       string        `gorm:"type:text;not null"`
	Amount        float64       `gorm:"type:numeric(15,2);not null"`
	Currency      string        `gorm:"type:text;not null"`
	Rank          string        `gorm:"type:text"`
	State         payment.State `gorm:"type:text;not null"`
	RetryCount    int           `gorm:"not null;default:0"`
	LastError     string        `gorm:"type:text"`
	NextRetryAt   *time.Time
	CreatedAt     time.Time `gorm:"not null;default:now()"`
	UpdatedAt     time.Time `gorm:"not null;default:now()"`
}

func (Model) TableName() string { return "public.payment_sagas" }

func (m Model) Snapshot() payment.Snapshot {
	return payment.Snapshot{
		TransactionID: m.TransactionID, AgentID: m.AgentID, Amount: m.Amount,
		Currency: m.Currency, Rank: m.Rank, State: m.State, LastError: m.LastError,
	}
}

func ApplySnapshot(model *Model, snapshot payment.Snapshot) {
	model.TransactionID = snapshot.TransactionID
	model.AgentID = snapshot.AgentID
	model.Amount = snapshot.Amount
	model.Currency = snapshot.Currency
	model.Rank = snapshot.Rank
	model.State = snapshot.State
	model.LastError = snapshot.LastError
}
