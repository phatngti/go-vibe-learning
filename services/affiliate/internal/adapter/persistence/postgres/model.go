package postgres

import "time"

type ReferralRow struct {
	ReferrerID string    `gorm:"type:text;not null"`
	RefereeID  string    `gorm:"type:text;not null;primaryKey"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`
}

func (ReferralRow) TableName() string { return "public.referrals" }

type AgentMetricRow struct {
	AgentID           string    `gorm:"type:text;primaryKey"`
	AccumulatedVolume float64   `gorm:"type:numeric(15,2);not null;default:0"`
	ReferralCount     int32     `gorm:"type:integer;not null;default:0"`
	UpdatedAt         time.Time `gorm:"not null;default:now()"`
}

func (AgentMetricRow) TableName() string { return "public.agent_metrics" }

type AffiliateLevelRow struct {
	AgentID     string    `gorm:"type:text;primaryKey"`
	CurrentRank string    `gorm:"type:text;not null;default:'BRONZE'"`
	PromotedAt  time.Time `gorm:"not null;default:now()"`
}

func (AffiliateLevelRow) TableName() string { return "public.affiliate_levels" }
