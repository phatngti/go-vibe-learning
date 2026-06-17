package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github/master_go_cqrs/internal/agent/domain"
	agentpostgres "github/master_go_cqrs/internal/agent/repository/postgres"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCallbackDeliveredAndReplayDoesNotRedeliver(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Idempotency-Key"); got != "tx-delivered" {
			t.Errorf("Idempotency-Key = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	handler, db := newCallbackTestHandler(t)
	createAgentConfig(t, db, "agent-1", server.URL)

	msg := callbackMessage("first", "tx-delivered", "agent-1")
	if err := handler.Handle(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), callbackMessage("replay", "tx-delivered", "agent-1")); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("callback calls = %d, want 1", calls)
	}
	assertDelivery(t, db, "tx-delivered", "DELIVERED", 1)
	assertCallbackEvent(t, db, "tx-delivered", messagebus.CallbackDispatchedType)
}

func TestCallbackSkippedWhenNotConfigured(t *testing.T) {
	handler, db := newCallbackTestHandler(t)
	if err := handler.Handle(context.Background(), callbackMessage("skip", "tx-skipped", "missing-agent")); err != nil {
		t.Fatal(err)
	}
	assertDelivery(t, db, "tx-skipped", "SKIPPED", 0)
	assertCallbackEvent(t, db, "tx-skipped", messagebus.CallbackDispatchSkippedType)
}

func TestCallbackRetriesServerFailuresThenPublishesTerminalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	handler, db := newCallbackTestHandler(t)
	createAgentConfig(t, db, "agent-2", server.URL)

	for attempt := 1; attempt <= maxCallbackAttempts; attempt++ {
		err := handler.Handle(context.Background(), callbackMessage(fmt.Sprintf("retry-%d", attempt), "tx-retry", "agent-2"))
		if attempt < maxCallbackAttempts && err == nil {
			t.Fatalf("attempt %d returned nil, want retryable error", attempt)
		}
		if attempt == maxCallbackAttempts && err != nil {
			t.Fatalf("terminal attempt error = %v", err)
		}
	}
	assertDelivery(t, db, "tx-retry", "FAILED", maxCallbackAttempts)
	assertCallbackEvent(t, db, "tx-retry", messagebus.CallbackDispatchFailedType)
}

func TestCallbackClientFailureIsTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	handler, db := newCallbackTestHandler(t)
	createAgentConfig(t, db, "agent-3", server.URL)

	if err := handler.Handle(context.Background(), callbackMessage("bad-request", "tx-bad", "agent-3")); err != nil {
		t.Fatal(err)
	}
	assertDelivery(t, db, "tx-bad", "FAILED", 1)
	assertCallbackEvent(t, db, "tx-bad", messagebus.CallbackDispatchFailedType)
}

func newCallbackTestHandler(t *testing.T) (*CallbackCommandHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:callback-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE public.agent_configs (
			agent_id TEXT PRIMARY KEY, callback_url TEXT NOT NULL, api_key TEXT NOT NULL UNIQUE,
			allowed_ips TEXT, updated_at DATETIME
		)`,
		`CREATE TABLE public.callback_deliveries (
			transaction_id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, callback_url TEXT,
			status TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT, updated_at DATETIME
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
	cfg.Kafka.Topics.AgentCommands = "agent.commands"
	cfg.Kafka.Topics.AgentEvents = "agent.events"
	platformDB := &platformpostgres.DB{DB: db}
	handler := NewCallbackCommandHandler(
		platformDB,
		agentpostgres.NewAgentRepository(platformDB),
		inbox.NewRepository(),
		outbox.NewRepository(db),
		cfg,
	)
	return handler, db
}

func createAgentConfig(t *testing.T, db *gorm.DB, agentID, callbackURL string) {
	t.Helper()
	if err := db.Create(&domain.AgentConfig{AgentID: agentID, CallbackURL: callbackURL, ApiKey: "key-" + agentID}).Error; err != nil {
		t.Fatal(err)
	}
}

func callbackMessage(id, transactionID, agentID string) *kafka.Message {
	payload, _ := json.Marshal(messagebus.DispatchCallbackRequested{
		TransactionID: transactionID, AgentID: agentID, Amount: 100, Currency: "USD",
	})
	return &kafka.Message{ID: id, EventType: messagebus.DispatchCallbackRequestedType, Payload: payload}
}

func assertDelivery(t *testing.T, db *gorm.DB, transactionID, status string, attempts int) {
	t.Helper()
	var delivery domain.CallbackDelivery
	if err := db.First(&delivery, "transaction_id = ?", transactionID).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Status != status || delivery.Attempts != attempts {
		t.Fatalf("delivery = %+v, want status %s attempts %d", delivery, status, attempts)
	}
}

func assertCallbackEvent(t *testing.T, db *gorm.DB, transactionID, eventType string) {
	t.Helper()
	var event outbox.Event
	if err := db.First(&event, "message_id = ?", messagebus.MessageID(transactionID, "callback-result")).Error; err != nil {
		t.Fatal(err)
	}
	if event.MessageType != eventType {
		t.Fatalf("event type = %s, want %s", event.MessageType, eventType)
	}
}
