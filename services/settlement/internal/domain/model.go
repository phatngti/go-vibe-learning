package domain

import (
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

// AgentBalance is the aggregate root for settlement.
type AgentBalance struct {
	AgentID            string
	ActiveBalance      float64
	CreditLimit        float64
	ActiveBalanceMoney *sharedmoney.Money
	CreditLimitMoney   *sharedmoney.Money
	UpdatedAt          time.Time
}

// CommissionLog represents a ledger entry for commission payments.
type CommissionLog struct {
	ID                string
	AgentID           string
	TransactionID     string
	TransactionAmount float64
	CommissionAmount  float64
	TransactionMoney  *sharedmoney.Money
	CommissionMoney   *sharedmoney.Money
	Rank              string
	CreatedAt         time.Time
}
