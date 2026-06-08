package domain

import (
	"context"
)

type GatewayChargeRequest struct {
	TransactionID string
	Amount        float64
	Currency      string
	CardNumber    string
	CVV           string
}

type GatewayChargeResponse struct {
	Success bool
	Message string
}

type PaymentGateway interface {
	Charge(ctx context.Context, req *GatewayChargeRequest) (*GatewayChargeResponse, error)
}
