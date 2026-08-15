package port

import (
	"context"
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type ProviderPaymentStatus string

const (
	ProviderStatusRequiresPaymentMethod ProviderPaymentStatus = "REQUIRES_PAYMENT_METHOD"
	ProviderStatusRequiresConfirmation  ProviderPaymentStatus = "REQUIRES_CONFIRMATION"
	ProviderStatusRequiresAction        ProviderPaymentStatus = "REQUIRES_ACTION"
	ProviderStatusProcessing            ProviderPaymentStatus = "PROCESSING"
	ProviderStatusSucceeded             ProviderPaymentStatus = "SUCCEEDED"
	ProviderStatusCanceled              ProviderPaymentStatus = "CANCELED"
	ProviderStatusFailed                ProviderPaymentStatus = "FAILED"
	ProviderStatusUnknown               ProviderPaymentStatus = "UNKNOWN"
)

type CreateIntentRequest struct {
	TransactionID     string
	IdempotencyKey    string
	Money             sharedmoney.Money
	CustomerReference string
	Metadata          map[string]string
}

type PaymentIntent struct {
	ProviderReference string
	ClientSecret      string
	Status            ProviderPaymentStatus
	Money             sharedmoney.Money
	TransactionID     string
	FailureReason     string
}

type ProviderWebhookEvent struct {
	Provider          string
	EventID           string
	EventType         string
	TransactionID     string
	ProviderReference string
	Status            ProviderPaymentStatus
	FailureReason     string
	PayloadSHA256     string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

// PaymentV2Repository extends the legacy transaction repository with the
// provider intent and webhook operations needed by Payment API v2. Keeping
// these methods on a separate port avoids coupling the legacy CQRS handlers to
// provider-specific lifecycle concerns.
type PaymentV2Repository interface {
	TransactionRepository
	SaveProviderIntent(ctx context.Context, id, provider, providerReference, providerStatus string) error
	TryRecordProviderEvent(ctx context.Context, event ProviderWebhookEvent) (bool, error)
	ApplyProviderOutcome(ctx context.Context, transactionID, provider, providerReference, providerStatus string, targetState *domain.TransactionState, lastError string) (*domain.Transaction, bool, error)
	MarkProviderEventProcessed(ctx context.Context, provider, eventID, processingStatus, lastError string) error
}

// PaymentProvider is owned by the application layer. Concrete provider SDK or
// HTTP types must never cross this boundary.
type PaymentProvider interface {
	Name() string
	CreateIntent(context.Context, CreateIntentRequest) (*PaymentIntent, error)
	RetrieveIntent(context.Context, string) (*PaymentIntent, error)
	CancelIntent(context.Context, string) (*PaymentIntent, error)
	ParseWebhook(payload []byte, signature string, receivedAt time.Time) (*ProviderWebhookEvent, error)
}
