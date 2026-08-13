package simulator

import (
	"context"
	"errors"

	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type simulatorGateway struct{}

func NewSimulatorGateway() port.PaymentGateway {
	return &simulatorGateway{}
}

func (g *simulatorGateway) Charge(ctx context.Context, req *domain.GatewayChargeRequest) (*domain.GatewayChargeResponse, error) {
	if req.CVV == "000" {
		return &domain.GatewayChargeResponse{
			Success: false,
			Message: "Simulated payment failure (CVV 000)",
		}, nil
	}

	if req.CardNumber == "4000000000000000" {
		return nil, errors.New("simulated processor timeout/system failure")
	}

	return &domain.GatewayChargeResponse{
		Success: true,
		Message: "Simulated payment processed successfully",
	}, nil
}
