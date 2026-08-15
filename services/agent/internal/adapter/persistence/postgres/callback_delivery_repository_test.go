package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCallbackScheduleIsAtomicAndIdempotent(t *testing.T) {
	repository, db := newCallbackRepositoryTest(t)
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	params := port.ScheduleCallbackParams{
		ConsumerName: "agent-service", MessageID: "event-1", Topic: "agent.commands", MessageType: "DispatchCallbackRequested",
		Delivery: &domain.CallbackDelivery{
			TransactionID: "tx-1", AgentID: "agent-1", TenantID: "tenant-1", EventID: "event-1",
			CallbackURL: "https://merchant.example/callback", Payload: []byte(`{"ok":true}`),
			Status: domain.CallbackDeliveryPending, NextAttemptAt: &now, OutcomeGeneration: 1,
		},
	}
	stored, created, err := repository.Schedule(context.Background(), params)
	if err != nil || !created || stored.EventID != "event-1" {
		t.Fatalf("stored=%+v created=%v err=%v", stored, created, err)
	}
	stored, created, err = repository.Schedule(context.Background(), params)
	if err != nil || created || stored.EventID != "event-1" {
		t.Fatalf("duplicate stored=%+v created=%v err=%v", stored, created, err)
	}
	assertTableCount(t, db, "public.callback_deliveries", 1)
	assertTableCount(t, db, "public.inbox_messages", 1)
}

func TestCallbackTerminalAndReplayPreserveEventIdentity(t *testing.T) {
	repository, db := newCallbackRepositoryTest(t)
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	delivery := &domain.CallbackDelivery{
		TransactionID: "tx-terminal", AgentID: "agent-1", TenantID: "tenant-1", EventID: "event-terminal",
		CallbackURL: "https://merchant.example/callback", Payload: []byte(`{}`),
		Status: domain.CallbackDeliveryInFlight, LeaseOwner: "worker-1", LeaseToken: 1,
		OutcomeGeneration: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(callbackToRow(delivery)).Error; err != nil {
		t.Fatal(err)
	}
	result := callbackRepositoryResult("tx-terminal", "event-terminal", 1)
	if err := repository.MarkTerminal(context.Background(), delivery, "exhausted", result, now); err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, db, "public.callback_dead_letters", 1)
	assertTableCount(t, db, "public.outbox_events", 1)

	replayed, err := repository.ReplayTerminal(context.Background(), port.CallbackReplayRequest{
		EventID: "event-terminal", TenantID: "tenant-1", OperatorID: "operator-1",
		Reason: "merchant endpoint recovered", AvailableAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.EventID != "event-terminal" || replayed.Status != domain.CallbackDeliveryRetryScheduled ||
		replayed.OutcomeGeneration != 2 || replayed.ReplayCount != 1 {
		t.Fatalf("replayed = %+v", replayed)
	}
	var dead CallbackDeadLetterRow
	if err := db.First(&dead, "event_id = ?", "event-terminal").Error; err != nil {
		t.Fatal(err)
	}
	if dead.Status != domain.CallbackDeadLetterReplayed || dead.ReplayedBy != "operator-1" {
		t.Fatalf("dead letter = %+v", dead)
	}
	assertTableCount(t, db, "public.agent_audit_logs", 1)
}

func TestCallbackLeaseFencingOnRetryAndTerminal(t *testing.T) {
	repository, db := newCallbackRepositoryTest(t)
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	delivery := &domain.CallbackDelivery{
		TransactionID: "tx-lease", AgentID: "agent-1", TenantID: "tenant-1", EventID: "event-lease",
		CallbackURL: "https://merchant.example/callback", Payload: []byte(`{}`),
		Status: domain.CallbackDeliveryInFlight, LeaseOwner: "worker-1", LeaseToken: 3, OutcomeGeneration: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(callbackToRow(delivery)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkRetry(context.Background(), delivery, "temporary", now.Add(30*time.Second), now); err != nil {
		t.Fatal(err)
	}
	var stored CallbackDeliveryRow
	if err := db.First(&stored, "transaction_id = ?", delivery.TransactionID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != domain.CallbackDeliveryRetryScheduled || stored.GenerationAttempts != 1 || stored.Attempts != 1 {
		t.Fatalf("retry state = %+v", stored)
	}
	if err := repository.MarkRetry(context.Background(), delivery, "stale", now.Add(time.Minute), now); err != domain.ErrCallbackDeliveryLeaseLost {
		t.Fatalf("stale lease error = %v", err)
	}

	if err := db.Model(&CallbackDeliveryRow{}).Where("transaction_id = ?", delivery.TransactionID).
		Updates(map[string]any{"status": domain.CallbackDeliveryInFlight, "lease_owner": "worker-2", "lease_token": 4}).Error; err != nil {
		t.Fatal(err)
	}
	delivery.LeaseOwner = "worker-2"
	delivery.LeaseToken = 4
	delivery.GenerationAttempts = 1
	result := callbackRepositoryResult(delivery.TransactionID, delivery.EventID, 1)
	if err := repository.MarkTerminal(context.Background(), delivery, "exhausted", result, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, db, "public.callback_dead_letters", 1)
	assertTableCount(t, db, "public.outbox_events", 1)
}

func newCallbackRepositoryTest(t *testing.T) (port.CallbackDeliveryRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:callback-repository-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE public.callback_deliveries (
			transaction_id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, tenant_id TEXT, event_id TEXT NOT NULL UNIQUE,
			callback_url TEXT, payload BLOB NOT NULL, key_version TEXT NOT NULL DEFAULT '', status TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0, generation_attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME, last_attempt_at DATETIME, lease_owner TEXT NOT NULL DEFAULT '',
			lease_token INTEGER NOT NULL DEFAULT 0, lease_expires_at DATETIME, replay_count INTEGER NOT NULL DEFAULT 0,
			outcome_generation INTEGER NOT NULL DEFAULT 1, last_error TEXT, created_at DATETIME,
			updated_at DATETIME, terminal_at DATETIME
		)`,
		`CREATE TABLE public.callback_dead_letters (
			id INTEGER PRIMARY KEY AUTOINCREMENT, event_id TEXT NOT NULL, transaction_id TEXT NOT NULL,
			agent_id TEXT NOT NULL, tenant_id TEXT NOT NULL, outcome_generation INTEGER NOT NULL,
			attempts INTEGER NOT NULL, reason TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME,
			replayed_at DATETIME, replayed_by TEXT NOT NULL DEFAULT '', replay_reason TEXT NOT NULL DEFAULT '',
			UNIQUE(event_id, outcome_generation)
		)`,
		`CREATE TABLE public.inbox_messages (
			consumer_name TEXT NOT NULL, message_id TEXT NOT NULL, topic TEXT NOT NULL, message_type TEXT NOT NULL,
			status TEXT NOT NULL, created_at DATETIME, processed_at DATETIME,
			PRIMARY KEY (consumer_name, message_id)
		)`,
		`CREATE TABLE public.outbox_events (
			id TEXT PRIMARY KEY, message_id TEXT NOT NULL UNIQUE, topic TEXT NOT NULL, message_key TEXT NOT NULL,
			message_type TEXT NOT NULL, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL, payload TEXT NOT NULL,
			schema_version INTEGER, aggregate_version INTEGER, tenant_id TEXT, correlation_id TEXT, causation_id TEXT,
			orchestration_owner TEXT, orchestration_generation INTEGER, traceparent TEXT, tracestate TEXT,
			status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, available_at DATETIME, locked_at DATETIME,
			locked_by TEXT, error_log TEXT, created_at DATETIME, processed_at DATETIME, dead_at DATETIME
		)`,
		`CREATE TABLE public.agent_audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT NOT NULL, tenant_id TEXT, action TEXT NOT NULL,
			details TEXT, created_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	platformDB := &platformpostgres.DB{DB: db}
	return NewCallbackDeliveryRepository(platformDB, inbox.NewRepository(), outbox.NewRepository(db)), db
}

func callbackRepositoryResult(transactionID, eventID string, generation int64) port.CallbackResultIntent {
	return port.CallbackResultIntent{
		MessageID: fmt.Sprintf("%s:callback-result:g%d", transactionID, generation), Topic: "agent.events",
		MessageKey: transactionID, MessageType: "CallbackDispatchFailed", AggregateType: "CallbackDelivery",
		AggregateID: transactionID, TenantID: "tenant-1", CausationID: eventID, Payload: []byte(`{"status":"failed"}`),
	}
}

func assertTableCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}
