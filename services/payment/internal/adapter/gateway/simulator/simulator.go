package simulator

import (
	"context"
	"errors"
	"sync"

	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type simulatorGateway struct {
	mu      sync.Mutex
	results map[string]*domain.GatewayLookupResponse
}

func NewSimulatorGateway() port.PaymentGateway {
	return &simulatorGateway{results: make(map[string]*domain.GatewayLookupResponse)}
}

func (g *simulatorGateway) Charge(ctx context.Context, req *domain.GatewayChargeRequest) (*domain.GatewayChargeResponse, error) {
	if req.CVV == "000" {
		result := &domain.GatewayChargeResponse{
			Success:   false,
			Message:   "Simulated payment failure (CVV 000)",
			Reference: "sim-declined-" + req.TransactionID,
		}
		g.remember(req.IdempotencyKey, result)
		return result, nil
	}

	if req.CardNumber == "4000000000000000" {
		// Simulate a response lost after the provider accepted the charge.
		g.remember(req.IdempotencyKey, &domain.GatewayChargeResponse{Success: true, Reference: "sim-charge-" + req.IdempotencyKey})
		return nil, errors.New("simulated processor timeout/system failure")
	}

	result := &domain.GatewayChargeResponse{
		Success:   true,
		Message:   "Simulated payment processed successfully",
		Reference: "sim-charge-" + req.IdempotencyKey,
	}
	g.remember(req.IdempotencyKey, result)
	return result, nil
}

func (g *simulatorGateway) LookupCharge(_ context.Context, idempotencyKey, _ string) (*domain.GatewayLookupResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	result, ok := g.results[idempotencyKey]
	if !ok {
		return &domain.GatewayLookupResponse{Found: false}, nil
	}
	copy := *result
	return &copy, nil
}

func (g *simulatorGateway) remember(key string, result *domain.GatewayChargeResponse) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.results[key] = &domain.GatewayLookupResponse{
		Found: true, Completed: result.Success, Message: result.Message, Reference: result.Reference,
	}
}
