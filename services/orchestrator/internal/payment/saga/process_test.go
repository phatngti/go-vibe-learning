package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/orchestrator/internal/config"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"
	paymentsagamodel "github/master_go_cqrs/services/orchestrator/internal/payment/saga/model"
	paymentpostgres "github/master_go_cqrs/services/orchestrator/internal/payment/saga/repository/postgres"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSagaSubscriptionsUseIndependentConsumerGroups(t *testing.T) {
	cfg := &config.Config{}
	cfg.Kafka.Topics = config.KafkaTopics{
		PaymentInitiatedEvents: "payments.initiated", PaymentEvents: "payments.events", AffiliateEvents: "affiliate.events",
		SettlementEvents: "settlement.events", AgentEvents: "agent.events",
	}
	handler := &Handler{}

	subscriptions := []kafka.Subscription{
		NewPaymentInitiatedSubscription(handler, cfg),
		NewPaymentSubscription(handler, cfg),
		NewAffiliateSubscription(handler, cfg),
		NewSettlementSubscription(handler, cfg),
		NewAgentSubscription(handler, cfg),
	}
	groups := make(map[string]struct{}, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription.Topic == "" || subscription.GroupID == "" || subscription.Handler == nil {
			t.Fatalf("invalid subscription: %+v", subscription)
		}
		if _, exists := groups[subscription.GroupID]; exists {
			t.Fatalf("consumer group %q is shared across topic-specific readers", subscription.GroupID)
		}
		groups[subscription.GroupID] = struct{}{}
	}
}

func TestSagaStartsAtInitiatedAndSchedulesPaymentOutcomeCheck(t *testing.T) {
	handler, db := newSagaTestHandler(t)
	mustHandle(t, handler, message("payment-initiated", messagebus.PaymentInitiatedType, messagebus.PaymentInitiated{
		TransactionID: "tx-init", AgentID: "agent-1", Amount: 100, Currency: "USD",
	}))
	var saga paymentsagamodel.Model
	if err := db.First(&saga, "transaction_id = ?", "tx-init").Error; err != nil {
		t.Fatal(err)
	}
	if saga.State != payment.StateWaitingPayment || saga.RetryCount != 1 || saga.NextRetryAt == nil {
		t.Fatalf("saga = %+v", saga)
	}
}

func TestSagaTransitionsToCompleted(t *testing.T) {
	handler, db := newSagaTestHandler(t)

	mustHandle(t, handler, message("payment-1", messagebus.PaymentCompletedType, messagebus.PaymentCompleted{
		TransactionID: "tx-1", AgentID: "agent-1", Amount: 250, Currency: "USD",
	}))
	assertSagaState(t, db, "tx-1", payment.StateWaitingAffiliate)
	assertOutboxMessage(t, db, messagebus.MessageID("tx-1", messagebus.StepAffiliate), messagebus.AccumulateVolumeRequestedType)

	mustHandle(t, handler, message("affiliate-1", messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{
		TransactionID: "tx-1", AgentID: "agent-1", CurrentRank: "SILVER",
	}))
	assertSagaState(t, db, "tx-1", payment.StateWaitingSettlement)
	assertOutboxMessage(t, db, messagebus.MessageID("tx-1", messagebus.StepSettlement), messagebus.CreditCommissionRequestedType)

	mustHandle(t, handler, message("settlement-1", messagebus.CommissionCreditedType, messagebus.CommissionCredited{
		TransactionID: "tx-1", AgentID: "agent-1", CommissionAmount: 5, NewBalance: 5,
	}))
	assertSagaState(t, db, "tx-1", payment.StateWaitingCallback)
	assertOutboxMessage(t, db, messagebus.MessageID("tx-1", messagebus.StepCallback), messagebus.DispatchCallbackRequestedType)

	mustHandle(t, handler, message("callback-1", messagebus.CallbackDispatchedType, messagebus.CallbackResult{
		TransactionID: "tx-1", AgentID: "agent-1", Status: "DELIVERED",
	}))
	assertSagaState(t, db, "tx-1", payment.StateCompleted)
}

func TestSagaCallbackFailureIsTerminalWithoutFinancialCompensation(t *testing.T) {
	handler, db := newSagaTestHandler(t)
	mustHandle(t, handler, message("payment-2", messagebus.PaymentCompletedType, messagebus.PaymentCompleted{
		TransactionID: "tx-2", AgentID: "agent-2", Amount: 100, Currency: "USD",
	}))
	mustHandle(t, handler, message("affiliate-2", messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{
		TransactionID: "tx-2", AgentID: "agent-2", CurrentRank: "BRONZE",
	}))
	mustHandle(t, handler, message("settlement-2", messagebus.CommissionCreditedType, messagebus.CommissionCredited{
		TransactionID: "tx-2", AgentID: "agent-2",
	}))
	mustHandle(t, handler, message("callback-2", messagebus.CallbackDispatchFailedType, messagebus.CallbackResult{
		TransactionID: "tx-2", AgentID: "agent-2", Status: "FAILED", Reason: "timeout",
	}))

	var saga paymentsagamodel.Model
	if err := db.First(&saga, "transaction_id = ?", "tx-2").Error; err != nil {
		t.Fatal(err)
	}
	if saga.State != payment.StateCallbackFailed || saga.LastError != "timeout" || saga.NextRetryAt != nil {
		t.Fatalf("unexpected terminal saga: %+v", saga)
	}
}

func TestSagaIgnoresDuplicateAndCompletedReplay(t *testing.T) {
	handler, db := newSagaTestHandler(t)
	paymentMessage := message("payment-3", messagebus.PaymentCompletedType, messagebus.PaymentCompleted{
		TransactionID: "tx-3", AgentID: "agent-3", Amount: 100, Currency: "USD",
	})
	mustHandle(t, handler, paymentMessage)
	mustHandle(t, handler, paymentMessage)

	var sagaCount int64
	if err := db.Model(&paymentsagamodel.Model{}).Where("transaction_id = ?", "tx-3").Count(&sagaCount).Error; err != nil {
		t.Fatal(err)
	}
	if sagaCount != 1 {
		t.Fatalf("saga count = %d, want 1", sagaCount)
	}

	mustHandle(t, handler, message("affiliate-3", messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{
		TransactionID: "tx-3", AgentID: "agent-3", CurrentRank: "BRONZE",
	}))
	mustHandle(t, handler, message("settlement-3", messagebus.CommissionCreditedType, messagebus.CommissionCredited{
		TransactionID: "tx-3", AgentID: "agent-3",
	}))
	mustHandle(t, handler, message("callback-3", messagebus.CallbackDispatchSkippedType, messagebus.CallbackResult{
		TransactionID: "tx-3", AgentID: "agent-3", Status: "SKIPPED",
	}))
	mustHandle(t, handler, message("affiliate-replay", messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{
		TransactionID: "tx-3", AgentID: "agent-3", CurrentRank: "DIAMOND",
	}))
	assertSagaState(t, db, "tx-3", payment.StateCompleted)
}

func TestOutOfOrderEventRemainsRetryable(t *testing.T) {
	handler, _ := newSagaTestHandler(t)
	err := handler.Handle(context.Background(), message("affiliate-early", messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{
		TransactionID: "tx-missing", AgentID: "agent-1", CurrentRank: "BRONZE",
	}))
	if err == nil || kafka.IsPermanent(err) {
		t.Fatalf("error = %v, want retryable missing-saga error", err)
	}
}

func TestMalformedSagaEventIsPermanent(t *testing.T) {
	handler, _ := newSagaTestHandler(t)
	err := handler.Handle(context.Background(), &kafka.Message{
		ID: "bad", EventType: messagebus.PaymentCompletedType, Payload: []byte(`{"transaction_id":""}`),
	})
	if !kafka.IsPermanent(err) {
		t.Fatalf("error = %v, want permanent error", err)
	}
}

func newSagaTestHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:saga-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE public.payment_sagas (
			transaction_id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, amount NUMERIC NOT NULL, amount_minor INTEGER,
			currency TEXT NOT NULL, currency_exponent INTEGER, rank TEXT, state TEXT NOT NULL, resume_state TEXT NOT NULL DEFAULT '', retry_count INTEGER NOT NULL DEFAULT 0,
			orchestration_owner TEXT NOT NULL DEFAULT 'saga:v1', orchestration_generation INTEGER NOT NULL DEFAULT 1,
			last_error TEXT, next_retry_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE public.inbox_messages (
			consumer_name TEXT NOT NULL, message_id TEXT NOT NULL, topic TEXT NOT NULL,
			message_type TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME,
			processed_at DATETIME, PRIMARY KEY (consumer_name, message_id)
		)`,
		`CREATE TABLE public.outbox_events (
			id TEXT PRIMARY KEY, message_id TEXT NOT NULL UNIQUE, topic TEXT NOT NULL,
			message_key TEXT NOT NULL, message_type TEXT NOT NULL, aggregate_type TEXT NOT NULL,
			aggregate_id TEXT NOT NULL, payload TEXT NOT NULL, status TEXT NOT NULL,
			schema_version INTEGER NOT NULL DEFAULT 1, aggregate_version INTEGER NOT NULL DEFAULT 0,
			tenant_id TEXT NOT NULL DEFAULT '', correlation_id TEXT NOT NULL DEFAULT '', causation_id TEXT NOT NULL DEFAULT '',
			orchestration_owner TEXT NOT NULL DEFAULT '', orchestration_generation INTEGER NOT NULL DEFAULT 0,
			traceparent TEXT NOT NULL DEFAULT '', tracestate TEXT NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0, available_at DATETIME, locked_at DATETIME,
			locked_by TEXT, error_log TEXT, created_at DATETIME, processed_at DATETIME,
			dead_at DATETIME
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
	return NewHandler(process, inbox.NewRepository()), db
}

func message(id, eventType string, payload any) *kafka.Message {
	encoded, _ := json.Marshal(payload)
	return &kafka.Message{ID: id, EventType: eventType, Payload: encoded}
}

func mustHandle(t *testing.T, handler *Handler, msg *kafka.Message) {
	t.Helper()
	if err := handler.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle(%s) error = %v", msg.EventType, err)
	}
}

func assertSagaState(t *testing.T, db *gorm.DB, transactionID string, want payment.State) {
	t.Helper()
	var saga paymentsagamodel.Model
	if err := db.First(&saga, "transaction_id = ?", transactionID).Error; err != nil {
		t.Fatal(err)
	}
	if saga.State != want {
		t.Fatalf("saga state = %s, want %s", saga.State, want)
	}
	if saga.OrchestrationOwner != "saga:v1" || saga.OrchestrationGeneration != 1 {
		t.Fatalf("saga orchestration identity = %s generation %d", saga.OrchestrationOwner, saga.OrchestrationGeneration)
	}
}

func assertOutboxMessage(t *testing.T, db *gorm.DB, messageID, messageType string) {
	t.Helper()
	var event outbox.Event
	if err := db.First(&event, "message_id = ?", messageID).Error; err != nil {
		t.Fatal(err)
	}
	if event.MessageType != messageType {
		t.Fatalf("outbox message type = %s, want %s", event.MessageType, messageType)
	}
	if event.OrchestrationOwner != "saga:v1" || event.OrchestrationGeneration != 1 {
		t.Fatalf("outbox orchestration identity = %s generation %d", event.OrchestrationOwner, event.OrchestrationGeneration)
	}
}
