package command

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// transactionRepoStub captures the saved transaction for assertion.
type transactionRepoStub struct {
	saved *domain.Transaction
}

func (r *transactionRepoStub) Save(ctx context.Context, tx *domain.Transaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	r.saved = tx
	return nil
}

func (r *transactionRepoStub) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}

func (r *transactionRepoStub) UpdateStatus(ctx context.Context, id string, status domain.TransactionState) error {
	return nil
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

	handler := NewInitiatePaymentHandler(uow, repo, es)

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
