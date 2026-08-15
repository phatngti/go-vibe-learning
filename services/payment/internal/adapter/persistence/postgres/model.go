package postgres

import "time"

// TransactionRow is the GORM persistence model for the transactions table.
type TransactionRow struct {
	ID                    string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AgentID               string  `gorm:"type:text;not null"`
	CustomerID            string  `gorm:"type:text;not null"`
	Amount                float64 `gorm:"type:numeric(15,2);not null"`
	AmountMinor           *int64  `gorm:"type:bigint"`
	CurrencyExponent      *int32  `gorm:"type:smallint"`
	Currency              string  `gorm:"type:text;not null"`
	Status                string  `gorm:"type:text;not null;default:'PENDING'"`
	CallbackURL           string  `gorm:"type:text"`
	RequestIdempotencyKey string  `gorm:"type:text;not null;default:''"`
	RequestFingerprint    string  `gorm:"type:text;not null;default:''"`
	IdempotencyKey        string  `gorm:"type:text"`
	ProviderReference     string  `gorm:"type:text"`
	Provider              string  `gorm:"type:text;not null;default:''"`
	ProviderStatus        string  `gorm:"type:text;not null;default:''"`
	CallbackEndpointID    string  `gorm:"type:text;not null;default:''"`
	LastError             string  `gorm:"type:text"`
	ProcessingStartedAt   *time.Time
	CreatedAt             time.Time `gorm:"not null;default:now()"`
	UpdatedAt             time.Time `gorm:"not null;default:now()"`
}

func (TransactionRow) TableName() string {
	return "public.transactions"
}

type ProviderWebhookEventRow struct {
	Provider          string    `gorm:"type:text;primaryKey"`
	EventID           string    `gorm:"type:text;primaryKey"`
	EventType         string    `gorm:"type:text;not null"`
	TransactionID     string    `gorm:"type:text;not null"`
	ProviderReference string    `gorm:"type:text;not null"`
	ProviderStatus    string    `gorm:"type:text;not null"`
	PayloadSHA256     string    `gorm:"type:text;not null"`
	ProcessingStatus  string    `gorm:"type:text;not null"`
	LastError         string    `gorm:"type:text;not null"`
	OccurredAt        time.Time `gorm:"not null"`
	ReceivedAt        time.Time `gorm:"not null"`
	ProcessedAt       *time.Time
}

func (ProviderWebhookEventRow) TableName() string {
	return "public.provider_webhook_events"
}
