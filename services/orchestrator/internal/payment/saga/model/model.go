package model

import (
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
)

type Model struct {
	TransactionID           string        `gorm:"type:text;primaryKey"`
	AgentID                 string        `gorm:"type:text;not null"`
	Amount                  float64       `gorm:"type:numeric(15,2);not null"`
	AmountMinor             *int64        `gorm:"type:bigint"`
	Currency                string        `gorm:"type:text;not null"`
	CurrencyExponent        *int32        `gorm:"type:smallint"`
	Rank                    string        `gorm:"type:text"`
	State                   payment.State `gorm:"type:text;not null"`
	ResumeState             payment.State `gorm:"type:text"`
	OrchestrationOwner      string        `gorm:"type:text;not null;default:'saga:v1'"`
	OrchestrationGeneration int64         `gorm:"not null;default:1"`
	RetryCount              int           `gorm:"not null;default:0"`
	LastError               string        `gorm:"type:text"`
	NextRetryAt             *time.Time
	CreatedAt               time.Time `gorm:"not null;default:now()"`
	UpdatedAt               time.Time `gorm:"not null;default:now()"`
}

func (Model) TableName() string { return "public.payment_sagas" }

func (m Model) Snapshot() payment.Snapshot {
	snapshot := payment.Snapshot{
		TransactionID: m.TransactionID, AgentID: m.AgentID, Amount: m.Amount,
		Currency: m.Currency, Rank: m.Rank, State: m.State, ResumeState: m.ResumeState, LastError: m.LastError,
		OrchestrationOwner: m.OrchestrationOwner, OrchestrationGeneration: m.OrchestrationGeneration,
	}
	if m.AmountMinor != nil && m.CurrencyExponent != nil {
		snapshot.Money = &sharedmoney.Money{AmountMinor: *m.AmountMinor, Currency: m.Currency, Exponent: *m.CurrencyExponent}
	}
	return snapshot
}

func ApplySnapshot(model *Model, snapshot payment.Snapshot) {
	model.TransactionID = snapshot.TransactionID
	model.AgentID = snapshot.AgentID
	model.Amount = snapshot.Amount
	model.Currency = snapshot.Currency
	if snapshot.Money != nil {
		amountMinor := snapshot.Money.AmountMinor
		exponent := snapshot.Money.Exponent
		model.AmountMinor = &amountMinor
		model.Currency = snapshot.Money.Currency
		model.CurrencyExponent = &exponent
	} else {
		model.AmountMinor = nil
		model.CurrencyExponent = nil
	}
	model.Rank = snapshot.Rank
	model.State = snapshot.State
	model.ResumeState = snapshot.ResumeState
	model.LastError = snapshot.LastError
	model.OrchestrationOwner = snapshot.OrchestrationOwner
	model.OrchestrationGeneration = snapshot.OrchestrationGeneration
}
