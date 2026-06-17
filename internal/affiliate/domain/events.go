package domain

import "time"

// ReferralRegistered is raised when a new agent-to-agent referral is recorded.
type ReferralRegistered struct {
	ReferrerID string    `json:"referrer_id"`
	RefereeID  string    `json:"referee_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (ReferralRegistered) EventType() string { return "ReferralRegistered" }

// VolumeAccumulated is raised when a payment transaction contributes to an agent's affiliate volume.
type VolumeAccumulated struct {
	TransactionID string    `json:"transaction_id"`
	AgentID       string    `json:"agent_id"`
	Amount        float64   `json:"amount"`
	TotalVolume   float64   `json:"total_volume"`
	CurrentRank   Rank      `json:"current_rank"`
	RankPromoted  bool      `json:"rank_promoted"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func (VolumeAccumulated) EventType() string { return "VolumeAccumulated" }

// RankPromoted is raised when an agent's rank increases after volume accumulation.
type RankPromoted struct {
	AgentID    string    `json:"agent_id"`
	OldRank    Rank      `json:"old_rank"`
	NewRank    Rank      `json:"new_rank"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (RankPromoted) EventType() string { return "RankPromoted" }
