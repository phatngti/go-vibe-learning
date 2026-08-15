package port

import (
	"context"
	"time"

	"github/master_go_cqrs/services/payment/internal/domain"
)

// TransactionRepository provides access to the transactions store.
type TransactionRepository interface {
	CreateOrGet(ctx context.Context, tx *domain.Transaction) (created bool, err error)
	GetByID(ctx context.Context, id string) (*domain.Transaction, error)
	ClaimForProcessing(ctx context.Context, id, idempotencyKey string, staleBefore time.Time) (*domain.Transaction, bool, error)
	FinalizeProcessing(ctx context.Context, id, idempotencyKey string, status domain.TransactionState, providerReference, lastError string) error
	ListForReconciliation(ctx context.Context, staleBefore time.Time, limit int) ([]*domain.Transaction, error)
	ResolveReconciliation(ctx context.Context, id, idempotencyKey string, status domain.TransactionState, providerReference, lastError string) (bool, error)
}

// PaymentGateway abstracts the external payment processing system.
type PaymentGateway interface {
	Charge(ctx context.Context, req *domain.GatewayChargeRequest) (*domain.GatewayChargeResponse, error)
	LookupCharge(ctx context.Context, idempotencyKey, providerReference string) (*domain.GatewayLookupResponse, error)
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
