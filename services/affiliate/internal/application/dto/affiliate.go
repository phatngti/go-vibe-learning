package dto

import sharedmoney "github/master_go_cqrs/internal/shared/money"

var MessageCode = map[string]string{}

type RegisterReferral struct {
	ReferrerID string `json:"referrer_id" example:"agent-123"`
	RefereeID  string `json:"referee_id" example:"agent-456"`
}

type AccumulateVolume struct {
	TransactionID string             `json:"transaction_id,omitempty"`
	AgentID       string             `json:"agent_id" example:"agent-123"`
	Money         *sharedmoney.Money `json:"money,omitempty"`
	Amount        float64            `json:"amount" example:"1000"`
	Currency      string             `json:"currency,omitempty" example:"USD"`
}

type RegisterReferralResponse struct {
	Status string `json:"status" example:"linked"`
}

type RankResponse struct {
	AgentID     string             `json:"agent_id" example:"agent-123"`
	CurrentRank string             `json:"current_rank" example:"BRONZE"`
	TotalMoney  *sharedmoney.Money `json:"total_money,omitempty" swaggertype:"object"`
	TotalVolume float64            `json:"total_volume" example:"2500"`
	Referrals   int32              `json:"referrals" example:"3"`
}

type DownlineResponse struct {
	AgentID        string   `json:"agent_id" example:"agent-123"`
	RefereeIDs     []string `json:"referee_ids" example:"agent-456,agent-789"`
	TotalReferrals int      `json:"total_referrals" example:"2"`
}
