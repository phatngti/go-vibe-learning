package saga

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/orchestrator/features/payment"
	"github/master_go_cqrs/internal/orchestrator/features/payment/codec"
	paymentsagamodel "github/master_go_cqrs/internal/orchestrator/features/payment/saga/model"
	paymentpostgres "github/master_go_cqrs/internal/orchestrator/features/payment/saga/repository/postgres"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReconcilerRepublishesTimedOutStepAfterRestart(t *testing.T) {
	reconciler, db := newReconcilerTest(t)
	due := time.Now().Add(-time.Minute)
	saga := &paymentsagamodel.Model{
		TransactionID: "tx-retry", AgentID: "agent-1", Amount: 100, Currency: "USD",
		State: payment.StateWaitingAffiliate, RetryCount: 2, NextRetryAt: &due,
	}
	if err := db.Create(saga).Error; err != nil {
		t.Fatal(err)
	}

	reconciler.Reconcile(context.Background())

	var got paymentsagamodel.Model
	if err := db.First(&got, "transaction_id = ?", saga.TransactionID).Error; err != nil {
		t.Fatal(err)
	}
	if got.RetryCount != 3 || got.NextRetryAt == nil || !got.NextRetryAt.After(time.Now()) {
		t.Fatalf("unexpected retried saga: %+v", got)
	}
	var event outbox.Event
	if err := db.First(&event, "message_id = ?", messagebus.MessageID(saga.TransactionID, messagebus.StepAffiliate)).Error; err != nil {
		t.Fatal(err)
	}
	if event.MessageType != messagebus.AccumulateVolumeRequestedType {
		t.Fatalf("message type = %s", event.MessageType)
	}
}

func TestReconcilerMarksTerminalFailuresAtRetryLimit(t *testing.T) {
	reconciler, db := newReconcilerTest(t)
	due := time.Now().Add(-time.Minute)
	sagas := []*paymentsagamodel.Model{
		{TransactionID: "tx-financial-failed", AgentID: "agent-1", Amount: 100, Currency: "USD", State: payment.StateWaitingSettlement, RetryCount: payment.MaxAttempts, NextRetryAt: &due},
		{TransactionID: "tx-callback-failed", AgentID: "agent-1", Amount: 100, Currency: "USD", State: payment.StateWaitingCallback, RetryCount: payment.MaxAttempts, NextRetryAt: &due},
	}
	if err := db.Create(sagas).Error; err != nil {
		t.Fatal(err)
	}

	reconciler.Reconcile(context.Background())

	var financial paymentsagamodel.Model
	if err := db.First(&financial, "transaction_id = ?", "tx-financial-failed").Error; err != nil {
		t.Fatal(err)
	}
	if financial.State != payment.StateFailed || financial.NextRetryAt != nil {
		t.Fatalf("unexpected financial failure state: %+v", financial)
	}
	var callback paymentsagamodel.Model
	if err := db.First(&callback, "transaction_id = ?", "tx-callback-failed").Error; err != nil {
		t.Fatal(err)
	}
	if callback.State != payment.StateCallbackFailed || callback.NextRetryAt != nil {
		t.Fatalf("unexpected callback failure state: %+v", callback)
	}
}

func newReconcilerTest(t *testing.T) (*Reconciler, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:reconciler-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE public.payment_sagas (
			transaction_id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, amount NUMERIC NOT NULL,
			currency TEXT NOT NULL, rank TEXT, state TEXT NOT NULL, retry_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT, next_retry_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE public.inbox_messages (
			message_id TEXT PRIMARY KEY, topic TEXT NOT NULL, message_type TEXT NOT NULL,
			status TEXT NOT NULL, created_at DATETIME, processed_at DATETIME
		)`,
		`CREATE TABLE public.outbox_events (
			id TEXT PRIMARY KEY, message_id TEXT NOT NULL UNIQUE, topic TEXT NOT NULL,
			message_key TEXT NOT NULL, message_type TEXT NOT NULL, aggregate_type TEXT NOT NULL,
			aggregate_id TEXT NOT NULL, payload TEXT NOT NULL, status TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0, available_at DATETIME, locked_at DATETIME,
			locked_by TEXT, error_log TEXT, created_at DATETIME, processed_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	cfg.Kafka.Topics = config.KafkaTopics{
		PaymentEvents: "payments.events", AffiliateCommands: "affiliate.commands", AffiliateEvents: "affiliate.events",
		SettlementCommands: "settlement.commands", SettlementEvents: "settlement.events",
		AgentCommands: "agent.commands", AgentEvents: "agent.events",
	}
	repo := paymentpostgres.NewRepository(&platformpostgres.DB{DB: db})
	process := NewProcess(repo, outbox.NewRepository(db), codec.NewKafka(cfg))
	return NewReconciler(process, logger.Factory{}), db
}
