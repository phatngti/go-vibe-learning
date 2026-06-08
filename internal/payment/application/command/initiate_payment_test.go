package command

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/domain"

	"gorm.io/gorm"
)

type transactionRepoStub struct {
	saved *domain.Transaction
}

func (r *transactionRepoStub) Save(ctx context.Context, db *gorm.DB, tx *domain.Transaction) error {
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

func TestInitiatePaymentHandlerCreatesPendingTransaction(t *testing.T) {
	repo := &transactionRepoStub{}
	handler := NewInitiatePaymentHandler(repo)

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
}
