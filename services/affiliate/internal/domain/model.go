package domain

import (
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

// Rank represents an affiliate tier level.
type Rank string

const (
	RankBronze   Rank = "BRONZE"
	RankSilver   Rank = "SILVER"
	RankGold     Rank = "GOLD"
	RankPlatinum Rank = "PLATINUM"
	RankDiamond  Rank = "DIAMOND"
)

// Referral records a referrer→referee relationship.
type Referral struct {
	ReferrerID string
	RefereeID  string
	CreatedAt  time.Time
}

// AgentMetric tracks an agent's accumulated volume and referral count.
type AgentMetric struct {
	AgentID                string
	AccumulatedVolume      float64
	AccumulatedVolumeMoney *sharedmoney.Money
	ReferralCount          int32
	UpdatedAt              time.Time
}

// AffiliateLevel tracks an agent's current rank.
type AffiliateLevel struct {
	AgentID     string
	CurrentRank Rank
	PromotedAt  time.Time
}
