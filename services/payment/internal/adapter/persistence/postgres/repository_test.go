package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateOrGetIsIdempotentAndRejectsFingerprintConflict(t *testing.T) {
	db := newTransactionSQLiteDB(t)
	repo := &transactionRepository{db: db}
	first := &domain.Transaction{
		AgentID: "agent-1", CustomerID: "customer-1", Amount: 100, Currency: "USD",
		Status: domain.StatePending, RequestIdempotencyKey: "order-123", RequestFingerprint: "sha256:first",
	}
	created, err := repo.CreateOrGet(context.Background(), first)
	if err != nil || !created {
		t.Fatalf("first CreateOrGet() = created %t, error %v", created, err)
	}

	replay := &domain.Transaction{
		AgentID: "agent-1", CustomerID: "customer-1", Amount: 100, Currency: "USD",
		Status: domain.StatePending, RequestIdempotencyKey: "order-123", RequestFingerprint: "sha256:first",
	}
	created, err = repo.CreateOrGet(context.Background(), replay)
	if err != nil || created || replay.ID != first.ID {
		t.Fatalf("replay CreateOrGet() = created %t, transaction %+v, error %v", created, replay, err)
	}

	conflict := &domain.Transaction{
		AgentID: "agent-1", CustomerID: "customer-1", Amount: 200, Currency: "USD",
		Status: domain.StatePending, RequestIdempotencyKey: "order-123", RequestFingerprint: "sha256:different",
	}
	created, err = repo.CreateOrGet(context.Background(), conflict)
	if created || !errors.Is(err, domain.ErrInitiationIdempotencyConflict) {
		t.Fatalf("conflict CreateOrGet() = created %t, error %v", created, err)
	}
}

func TestProviderOutcomeIsMonotonicAcrossOutOfOrderDelivery(t *testing.T) {
	db := newTransactionSQLiteDB(t)
	repo := &transactionRepository{db: db}
	tx := &domain.Transaction{
		AgentID: "agent-1", CustomerID: "customer-1", Amount: 10.50, Currency: "USD",
		Status: domain.StatePending, RequestIdempotencyKey: "order-123", RequestFingerprint: "sha256:first",
	}
	created, err := repo.CreateOrGet(context.Background(), tx)
	if err != nil || !created {
		t.Fatalf("CreateOrGet() created=%v error=%v", created, err)
	}
	completed := domain.StateCompleted
	result, changed, err := repo.ApplyProviderOutcome(
		context.Background(), tx.ID, "stripe", "pi_123", string(port.ProviderStatusSucceeded), &completed, "",
	)
	if err != nil || !changed || result.Status != domain.StateCompleted {
		t.Fatalf("success outcome result=%+v changed=%v error=%v", result, changed, err)
	}

	failed := domain.StateFailed
	result, changed, err = repo.ApplyProviderOutcome(
		context.Background(), tx.ID, "stripe", "pi_123", string(port.ProviderStatusFailed), &failed, "late failure",
	)
	if err != nil || changed || result.Status != domain.StateCompleted || result.ProviderStatus != string(port.ProviderStatusSucceeded) {
		t.Fatalf("late failure result=%+v changed=%v error=%v", result, changed, err)
	}

	if err := repo.SaveProviderIntent(context.Background(), tx.ID, "stripe", "pi_123", string(port.ProviderStatusRequiresAction)); err != nil {
		t.Fatal(err)
	}
	result, err = repo.GetByID(context.Background(), tx.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.StateCompleted || result.ProviderStatus != string(port.ProviderStatusSucceeded) {
		t.Fatalf("late create response regressed terminal state: %+v", result)
	}
}

func newTransactionSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:payment-repo-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.transactions (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))), agent_id TEXT NOT NULL,
		customer_id TEXT NOT NULL, amount NUMERIC NOT NULL, amount_minor INTEGER,
		currency_exponent INTEGER, currency TEXT NOT NULL,
		status TEXT NOT NULL, callback_url TEXT, request_idempotency_key TEXT NOT NULL DEFAULT '',
		request_fingerprint TEXT NOT NULL DEFAULT '', idempotency_key TEXT,
		provider_reference TEXT, provider TEXT, provider_status TEXT,
		callback_endpoint_id TEXT, last_error TEXT, processing_started_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX public.uq_transactions_agent_request_idempotency
		ON transactions (agent_id, request_idempotency_key) WHERE request_idempotency_key <> ''`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
