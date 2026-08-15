package postgres

import "time"

type AgentBalanceRow struct {
	AgentID            string    `gorm:"type:text;primaryKey"`
	ActiveBalance      float64   `gorm:"type:numeric(15,2);not null;default:0"`
	CreditLimit        float64   `gorm:"type:numeric(15,2);not null;default:1000"`
	ActiveBalanceMinor *int64    `gorm:"type:bigint"`
	CreditLimitMinor   *int64    `gorm:"type:bigint"`
	Currency           *string   `gorm:"type:text"`
	CurrencyExponent   *int32    `gorm:"type:smallint"`
	UpdatedAt          time.Time `gorm:"not null;default:now()"`
}

func (AgentBalanceRow) TableName() string { return "public.agent_balances" }

type CommissionLogRow struct {
	ID                     string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID                string    `gorm:"type:text;not null"`
	TransactionID          string    `gorm:"type:text;not null"`
	TransactionAmount      float64   `gorm:"type:numeric(15,2);not null"`
	CommissionAmount       float64   `gorm:"type:numeric(15,2);not null"`
	TransactionAmountMinor *int64    `gorm:"type:bigint"`
	CommissionAmountMinor  *int64    `gorm:"type:bigint"`
	Currency               *string   `gorm:"type:text"`
	CurrencyExponent       *int32    `gorm:"type:smallint"`
	Rank                   string    `gorm:"type:text;not null"`
	CreatedAt              time.Time `gorm:"not null;default:now()"`
}

func (CommissionLogRow) TableName() string { return "public.commission_ledgers" }
