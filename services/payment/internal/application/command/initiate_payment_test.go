package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	paymentconfig "github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// transactionRepoStub captures the saved transaction for assertion.
type transactionRepoStub struct {
	saved    *domain.Transaction
	existing *domain.Transaction
}

func (r *transactionRepoStub) CreateOrGet(ctx context.Context, tx *domain.Transaction) (bool, error) {
	if r.existing != nil {
		*tx = *r.existing
		return false, nil
	}
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	r.saved = tx
	return true, nil
}

func (r *transactionRepoStub) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}

func (r *transactionRepoStub) ClaimForProcessing(context.Context, string, string, time.Time) (*domain.Transaction, bool, error) {
	return nil, false, nil
}

func (r *transactionRepoStub) FinalizeProcessing(context.Context, string, string, domain.TransactionState, string, string) error {
	return nil
}
func (r *transactionRepoStub) ListForReconciliation(context.Context, time.Time, int) ([]*domain.Transaction, error) {
	return nil, nil
}
func (r *transactionRepoStub) ResolveReconciliation(context.Context, string, string, domain.TransactionState, string, string) (bool, error) {
	return false, nil
}

// uowStub is a no-op UnitOfWork that just runs the function directly.
type uowStub struct{}

func (u *uowStub) Do(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

// eventStoreStub is a no-op event store for unit tests.
type eventStoreStub struct{ appended *eventstore.DomainEvent }

func (e *eventStoreStub) AppendEvent(_ context.Context, _ *gorm.DB, event *eventstore.DomainEvent) error {
	e.appended = event
	return nil
}

func (e *eventStoreStub) LoadEvents(_ context.Context, _, _ string) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}

func (e *eventStoreStub) LoadEventsFrom(_ context.Context, _, _ string, _ int64) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}

func (e *eventStoreStub) NextVersion(_ context.Context, _ *gorm.DB, _, _ string) (int64, error) {
	return 1, nil
}

func TestInitiatePaymentHandlerCreatesPendingTransaction(t *testing.T) {
	uow := &uowStub{}
	repo := &transactionRepoStub{}
	es := &eventStoreStub{}

	handler := NewInitiatePaymentHandler(uow, repo, es, new(paymentconfig.Config))

	tx, err := handler.Handle(context.Background(), &dto.InitiatePayment{
		AgentID:     "agent-1",
		Amount:      100,
		Currency:    "USD",
		CustomerID:  "customer-1",
		CallbackURL: "https://example.com/callback",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if repo.saved == nil {
		t.Fatal("expected transaction to be saved")
	}
	if tx.Status != domain.StatePending {
		t.Fatalf("expected pending status, got %s", tx.Status)
	}
	if tx.AgentID != "agent-1" || tx.CustomerID != "customer-1" || tx.CallbackURL != "https://example.com/callback" {
		t.Fatalf("transaction fields were not mapped correctly: %#v", tx)
	}
	if es.appended == nil || es.appended.EventType != (domain.PaymentInitiated{}).EventType() || es.appended.AggregateID != tx.ID {
		t.Fatalf("appended event = %+v", es.appended)
	}
}

func TestInitiatePaymentHandlerReplaysExistingRequestWithoutDuplicateEvent(t *testing.T) {
	existing := &domain.Transaction{ID: uuid.NewString(), AgentID: "agent-1", Amount: 100, Currency: "USD", Status: domain.StatePending}
	repo := &transactionRepoStub{existing: existing}
	es := &eventStoreStub{}
	handler := NewInitiatePaymentHandler(&uowStub{}, repo, es, new(paymentconfig.Config))

	tx, err := handler.Handle(context.Background(), &dto.InitiatePayment{
		AgentID: "agent-1", Amount: 100, Currency: "USD", IdempotencyKey: "order-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tx.ID != existing.ID {
		t.Fatalf("transaction ID = %s, want %s", tx.ID, existing.ID)
	}
	if es.appended != nil {
		t.Fatalf("duplicate request appended event: %+v", es.appended)
	}
}

func TestInitiatePaymentHandlerCanRequireIdempotencyKey(t *testing.T) {
	cfg := new(paymentconfig.Config)
	cfg.Initiation.RequireIdempotency = true
	handler := NewInitiatePaymentHandler(&uowStub{}, &transactionRepoStub{}, &eventStoreStub{}, cfg)
	_, err := handler.Handle(context.Background(), &dto.InitiatePayment{AgentID: "agent-1", Amount: 100, Currency: "USD"})
	if !errors.Is(err, domain.ErrInitiationIdempotencyRequired) {
		t.Fatalf("Handle() error = %v, want ErrInitiationIdempotencyRequired", err)
	}
}
