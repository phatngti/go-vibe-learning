package postgres

import "time"

// TransactionRow is the GORM persistence model for the transactions table.
type TransactionRow struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID     string    `gorm:"type:text;not null"`
	CustomerID  string    `gorm:"type:text;not null"`
	Amount      float64   `gorm:"type:numeric(15,2);not null"`
	Currency    string    `gorm:"type:text;not null"`
	Status      string    `gorm:"type:text;not null;default:'PENDING'"`
	CallbackURL string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (TransactionRow) TableName() string {
	return "public.transactions"
}
