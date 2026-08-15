package domain

import (
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

// TransactionState represents the lifecycle state of a payment transaction.
type TransactionState string

const (
	StatePending    TransactionState = "PENDING"
	StateProcessing TransactionState = "PROCESSING"
	StateCompleted  TransactionState = "COMPLETED"
	StateFailed     TransactionState = "FAILED"
	// StateUnknown means the provider outcome is ambiguous (for example, a
	// timeout after the provider may already have accepted the charge). It must
	// be reconciled or retried with the same provider idempotency key.
	StateUnknown TransactionState = "UNKNOWN"
)

// Transaction is the aggregate root for a payment transaction.
type Transaction struct {
	ID                    string
	AgentID               string
	CustomerID            string
	Amount                float64
	AmountMinor           int64
	CurrencyExponent      int32
	Currency              string
	Status                TransactionState
	CallbackURL           string
	RequestIdempotencyKey string
	RequestFingerprint    string
	IdempotencyKey        string
	ProviderReference     string
	Provider              string
	ProviderStatus        string
	CallbackEndpointID    string
	LastError             string
	ProcessingStartedAt   *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GatewayChargeRequest holds the data needed to charge a payment gateway.
type GatewayChargeRequest struct {
	TransactionID  string
	Amount         float64
	Currency       string
	CardNumber     string
	CVV            string
	IdempotencyKey string
}

// GatewayChargeResponse holds the result of a payment gateway charge.
type GatewayChargeResponse struct {
	Success   bool
	Message   string
	Reference string
}

// GatewayLookupResponse represents a provider-side lookup. Found=false means
// the provider still cannot supply an authoritative outcome and no local
// terminal transition is allowed.
type GatewayLookupResponse struct {
	Found     bool
	Completed bool
	Message   string
	Reference string
}

// MonetaryAmount returns the minor-unit representation, falling back to the
// legacy decimal field only for records that have not been backfilled yet.
func (t Transaction) MonetaryAmount() (sharedmoney.Money, error) {
	if t.AmountMinor != 0 {
		return sharedmoney.New(t.AmountMinor, t.Currency, t.CurrencyExponent)
	}
	return sharedmoney.FromLegacyFloat64(t.Amount, t.Currency)
}
