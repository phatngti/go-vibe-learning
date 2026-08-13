package port

import (
	"context"

	"github/master_go_cqrs/services/payment/internal/domain"
)

// TransactionRepository provides access to the transactions store.
type TransactionRepository interface {
	Save(ctx context.Context, tx *domain.Transaction) error
	GetByID(ctx context.Context, id string) (*domain.Transaction, error)
	UpdateStatus(ctx context.Context, id string, status domain.TransactionState) error
}

// PaymentGateway abstracts the external payment processing system.
type PaymentGateway interface {
	Charge(ctx context.Context, req *domain.GatewayChargeRequest) (*domain.GatewayChargeResponse, error)
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
