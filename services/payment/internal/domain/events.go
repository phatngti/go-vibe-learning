package domain

import "time"

type Event interface {
	EventType() string
}

// PaymentInitiated is raised when a new payment transaction is created.
type PaymentInitiated struct {
	TransactionID string    `json:"transaction_id"`
	AgentID       string    `json:"agent_id"`
	CustomerID    string    `json:"customer_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	CallbackURL   string    `json:"callback_url"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// EventType returns the canonical event type name.
func (PaymentInitiated) EventType() string { return "PaymentInitiated" }

// PaymentCompleted is raised when a payment transaction succeeds.
type PaymentCompleted struct {
	TransactionID string    `json:"transaction_id"`
	AgentID       string    `json:"agent_id"`
	CustomerID    string    `json:"customer_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// EventType returns the canonical event type name.
func (PaymentCompleted) EventType() string { return "PaymentCompleted" }

// PaymentFailed is raised when a payment transaction fails.
type PaymentFailed struct {
	TransactionID string    `json:"transaction_id"`
	AgentID       string    `json:"agent_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// EventType returns the canonical event type name.
func (PaymentFailed) EventType() string { return "PaymentFailed" }
