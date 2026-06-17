package command

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/domain"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// transactionRepoStub captures the saved transaction for assertion.
type transactionRepoStub struct {
	saved *domain.Transaction
}

func (r *transactionRepoStub) Save(ctx context.Context, db *gorm.DB, tx *domain.Transaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	r.saved = tx
	return nil
}

func (r *transactionRepoStub) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}

func (r *transactionRepoStub) GetByIDWithDB(ctx context.Context, db *gorm.DB, id string) (*domain.Transaction, error) {
	return nil, nil
}

func (r *transactionRepoStub) UpdateStatus(ctx context.Context, db *gorm.DB, id string, status domain.TransactionState) error {
	return nil
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
	// Use an in-memory SQLite DB so the handler's db.Transaction() call works.
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	db := &platformpostgres.DB{DB: sqliteDB}
	repo := &transactionRepoStub{}
	es := &eventStoreStub{}

	handler := NewInitiatePaymentHandler(db, repo, es)

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
