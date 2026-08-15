package dto

import sharedmoney "github/master_go_cqrs/internal/shared/money"

var MessageCode = map[string]string{}

type InitiatePayment struct {
	AgentID        string  `json:"agent_id" example:"agent-123"`
	Amount         float64 `json:"amount" example:"100.50"`
	Currency       string  `json:"currency" example:"USD"`
	CustomerID     string  `json:"customer_id" example:"customer-123"`
	CallbackURL    string  `json:"callback_url" example:"https://merchant.example/callback"`
	IdempotencyKey string  `json:"idempotency_key,omitempty" example:"merchant-order-20260813-123"`
}

type ProcessPayment struct {
	TransactionID  string `json:"transaction_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	PaymentMethod  string `json:"payment_method" example:"credit_card"`
	CardNumber     string `json:"card_number" example:"4111111111111111"`
	CVV            string `json:"cvv" example:"123"`
	IdempotencyKey string `json:"idempotency_key,omitempty" example:"checkout-550e8400-attempt-1"`
}

type ProcessCheckoutRequest struct {
	CardNumber     string `json:"card_number" example:"4111111111111111"`
	CVV            string `json:"cvv" example:"123"`
	IdempotencyKey string `json:"idempotency_key,omitempty" example:"checkout-550e8400-attempt-1"`
}

type TransactionResponse struct {
	ID          string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	AgentID     string  `json:"agent_id" example:"agent-123"`
	CustomerID  string  `json:"customer_id" example:"customer-123"`
	Amount      float64 `json:"amount" example:"100.50"`
	Currency    string  `json:"currency" example:"USD"`
	Status      string  `json:"status" example:"PENDING"`
	CallbackURL string  `json:"callback_url" example:"https://merchant.example/callback"`
	CreatedAt   string  `json:"created_at" example:"2026-06-06T12:00:00+07:00"`
	UpdatedAt   string  `json:"updated_at" example:"2026-06-06T12:00:00+07:00"`
}

type CheckoutPageResponse struct {
	Message       string  `json:"message" example:"Welcome to Secure Checkout Page"`
	TransactionID string  `json:"transaction_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Amount        float64 `json:"amount" example:"100.50"`
	Currency      string  `json:"currency" example:"USD"`
	AgentID       string  `json:"agent_id" example:"agent-123"`
	Status        string  `json:"status" example:"PENDING"`
	Help          string  `json:"help" example:"Submit a POST to this same URL with payment details to pay"`
}

type CreatePaymentV2 struct {
	AgentID            string
	Money              sharedmoney.Money
	CustomerReference  string
	CallbackEndpointID string
	Metadata           map[string]string
	IdempotencyKey     string
}

type CreatePaymentV2Result struct {
	Transaction  *TransactionResponseV2
	Provider     string
	ClientSecret string
}

type TransactionResponseV2 struct {
	ID                 string
	AgentID            string
	CustomerReference  string
	Money              sharedmoney.Money
	Status             string
	Provider           string
	ProviderReference  string
	ProviderStatus     string
	CallbackEndpointID string
	CreatedAt          string
	UpdatedAt          string
}

func NewTransactionResponse(txID, agentID, customerID string, amount float64, currency, status, callbackURL, createdAt, updatedAt string) TransactionResponse {
	return TransactionResponse{
		ID:          txID,
		AgentID:     agentID,
		CustomerID:  customerID,
		Amount:      amount,
		Currency:    currency,
		Status:      status,
		CallbackURL: callbackURL,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
