package domain

import (
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

// CommissionCredited is raised when a commission is calculated and added to an agent's balance.
type CommissionCredited struct {
	AgentID           string             `json:"agent_id"`
	TransactionID     string             `json:"transaction_id"`
	TransactionMoney  *sharedmoney.Money `json:"transaction_money,omitempty"`
	CommissionMoney   *sharedmoney.Money `json:"commission_money,omitempty"`
	BalanceMoney      *sharedmoney.Money `json:"balance_money,omitempty"`
	TransactionAmount float64            `json:"transaction_amount"`
	CommissionAmount  float64            `json:"commission_amount"`
	Rank              string             `json:"rank"`
	NewBalance        float64            `json:"new_balance"`
	OccurredAt        time.Time          `json:"occurred_at"`
}

func (CommissionCredited) EventType() string { return "CommissionCredited" }
