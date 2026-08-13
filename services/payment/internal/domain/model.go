package domain

import "time"

// TransactionState represents the lifecycle state of a payment transaction.
type TransactionState string

const (
	StatePending    TransactionState = "PENDING"
	StateProcessing TransactionState = "PROCESSING"
	StateCompleted  TransactionState = "COMPLETED"
	StateFailed     TransactionState = "FAILED"
)

// Transaction is the aggregate root for a payment transaction.
type Transaction struct {
	ID          string
	AgentID     string
	CustomerID  string
	Amount      float64
	Currency    string
	Status      TransactionState
	CallbackURL string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GatewayChargeRequest holds the data needed to charge a payment gateway.
type GatewayChargeRequest struct {
	TransactionID string
	Amount        float64
	Currency      string
	CardNumber    string
	CVV           string
}

// GatewayChargeResponse holds the result of a payment gateway charge.
type GatewayChargeResponse struct {
	Success bool
	Message string
}
