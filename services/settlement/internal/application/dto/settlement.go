package dto

import sharedmoney "github/master_go_cqrs/internal/shared/money"

var MessageCode = map[string]string{}

type CalculateAndCreditCommission struct {
	AgentID           string             `json:"agent_id" example:"agent-123"`
	TransactionMoney  *sharedmoney.Money `json:"transaction_money,omitempty"`
	TransactionAmount float64            `json:"transaction_amount" example:"1000"`
	Currency          string             `json:"currency,omitempty" example:"USD"`
	Rank              string             `json:"rank" example:"BRONZE"`
	TransactionID     string             `json:"transaction_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type RequestWithdrawal struct {
	AgentID string  `json:"agent_id" example:"agent-123"`
	Amount  float64 `json:"amount" example:"100"`
}

type AdjustCreditLimit struct {
	AgentID string  `json:"agent_id" example:"agent-123"`
	Limit   float64 `json:"limit" example:"1000"`
}

type BalanceResponse struct {
	AgentID       string  `json:"agent_id" example:"agent-123"`
	ActiveBalance float64 `json:"active_balance" example:"500"`
	CreditLimit   float64 `json:"credit_limit" example:"1000"`
	MaxWithdrawal float64 `json:"max_withdrawal" example:"1500"`
}

type WithdrawalResponse struct {
	AgentID       string  `json:"agent_id" example:"agent-123"`
	Amount        float64 `json:"amount" example:"100"`
	Status        string  `json:"status" example:"SUCCESS"`
	ActiveBalance float64 `json:"active_balance" example:"400"`
}

type CreditLimitResponse struct {
	AgentID       string  `json:"agent_id" example:"agent-123"`
	ActiveBalance float64 `json:"active_balance" example:"500"`
	CreditLimit   float64 `json:"credit_limit" example:"1000"`
}
