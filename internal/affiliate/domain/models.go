package domain

import (
	"time"
)

type Rank string

const (
	RankBronze   Rank = "BRONZE"
	RankSilver   Rank = "SILVER"
	RankGold     Rank = "GOLD"
	RankPlatinum Rank = "PLATINUM"
	RankDiamond  Rank = "DIAMOND"
)

type Referral struct {
	ReferrerID string    `gorm:"type:text;not null"`
	RefereeID  string    `gorm:"type:text;not null;primaryKey"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
}

func (Referral) TableName() string {
	return "public.referrals"
}

type AgentMetric struct {
	AgentID           string    `gorm:"type:text;primaryKey"`
	AccumulatedVolume float64   `gorm:"type:numeric(15,2);not null;default:0"`
	ReferralCount     int32     `gorm:"type:integer;not null;default:0"`
	UpdatedAt         time.Time `gorm:"not null;default:now()"`
}

func (AgentMetric) TableName() string {
	return "public.agent_metrics"
}

type AffiliateLevel struct {
	AgentID     string    `gorm:"type:text;primaryKey"`
	CurrentRank Rank      `gorm:"type:text;not null;default:'BRONZE'"`
	PromotedAt  time.Time `gorm:"not null;default:now()"`
}

func (AffiliateLevel) TableName() string {
	return "public.affiliate_levels"
}
