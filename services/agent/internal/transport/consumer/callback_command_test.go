package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	agentpostgres "github/master_go_cqrs/services/agent/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/agent/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCallbackDeliveredAndReplayDoesNotRedeliver(t *testing.T) {
	calls := 0
	handler, db := newCallbackTestHandler(t)
	handler.httpClient = stubHTTPClient(func(r *http.Request) int {
		calls++
		if got := r.Header.Get("Idempotency-Key"); got != "tx-delivered" {
			t.Errorf("Idempotency-Key = %q", got)
		}
		return http.StatusNoContent
	})
	createAgentConfig(t, db, "agent-1", "http://callback.test/delivered")

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
	handler, db := newCallbackTestHandler(t)
	handler.httpClient = stubHTTPClient(func(*http.Request) int { return http.StatusServiceUnavailable })
	createAgentConfig(t, db, "agent-2", "http://callback.test/retry")

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
	handler, db := newCallbackTestHandler(t)
	handler.httpClient = stubHTTPClient(func(*http.Request) int { return http.StatusBadRequest })
	createAgentConfig(t, db, "agent-3", "http://callback.test/bad-request")

	if err := handler.Handle(context.Background(), callbackMessage("bad-request", "tx-bad", "agent-3")); err != nil {
		t.Fatal(err)
	}
	assertDelivery(t, db, "tx-bad", "FAILED", 1)
	assertCallbackEvent(t, db, "tx-bad", messagebus.CallbackDispatchFailedType)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func stubHTTPClient(status func(*http.Request) int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status(request), Body: http.NoBody, Header: make(http.Header)}, nil
	})}
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

// agentConfigRow is a test helper for inserting agent config using GORM.
type agentConfigRow struct {
	AgentID     string `gorm:"type:text;primaryKey"`
	CallbackURL string `gorm:"type:text;not null"`
	ApiKey      string `gorm:"type:text;uniqueIndex;not null"`
	AllowedIPs  string `gorm:"type:text;default:''"`
}

func (agentConfigRow) TableName() string { return "public.agent_configs" }

func createAgentConfig(t *testing.T, db *gorm.DB, agentID, callbackURL string) {
	t.Helper()
	if err := db.Create(&agentConfigRow{AgentID: agentID, CallbackURL: callbackURL, ApiKey: "key-" + agentID}).Error; err != nil {
		t.Fatal(err)
	}
}

// callbackDeliveryRow is a test helper for reading callback deliveries via GORM.
type callbackDeliveryRow struct {
	TransactionID string `gorm:"primaryKey"`
	Status        string
	Attempts      int
}

func (callbackDeliveryRow) TableName() string { return "public.callback_deliveries" }

func callbackMessage(id, transactionID, agentID string) *kafka.Message {
	payload, _ := json.Marshal(messagebus.DispatchCallbackRequested{
		TransactionID: transactionID, AgentID: agentID, Amount: 100, Currency: "USD",
	})
	return &kafka.Message{ID: id, EventType: messagebus.DispatchCallbackRequestedType, Payload: payload}
}

func assertDelivery(t *testing.T, db *gorm.DB, transactionID, status string, attempts int) {
	t.Helper()
	var delivery callbackDeliveryRow
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
