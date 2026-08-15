// File: internal/platform/outbox/repository_test.go
// Description: Unit and mock tests for outbox repository operations.

package outbox

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnqueueKeepsExistingMessageImmutable(t *testing.T) {
	db := newOutboxSQLiteDB(t)
	repo := NewRepository(db)
	params := EnqueueParams{
		MessageID: "tx-1:affiliate", Topic: "affiliate.commands", MessageKey: "tx-1",
		MessageType: "AccumulateVolumeRequested", AggregateType: "PaymentSaga",
		AggregateID: "tx-1", Payload: []byte(`{"amount":100}`),
	}
	if err := repo.Enqueue(context.Background(), nil, params); err != nil {
		t.Fatal(err)
	}
	var first Event
	if err := db.First(&first, "message_id = ?", params.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Event{}).Where("id = ?", first.ID).Updates(map[string]any{"status": "PROCESSING", "locked_by": "worker-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkProcessed(context.Background(), first.ID, "worker-1"); err != nil {
		t.Fatal(err)
	}
	params.Payload = []byte(`{"amount":200}`)
	if err := repo.Enqueue(context.Background(), nil, params); err != nil {
		t.Fatal(err)
	}

	var events []Event
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Status != "PROCESSED" || events[0].ProcessedAt == nil || events[0].Payload != `{"amount":100}` {
		t.Fatalf("duplicate enqueue mutated the original event: %+v", events)
	}
}

func TestEnqueuePersistsEnvelopeMetadataWhenSchemaSupportsIt(t *testing.T) {
	db := newOutboxSQLiteDB(t)
	repo := NewRepository(db)
	params := EnqueueParams{
		MessageID: "tx-1:affiliate", Topic: "affiliate.commands", MessageKey: "tx-1",
		MessageType: "AccumulateVolumeRequested", AggregateType: "PaymentSaga", AggregateID: "tx-1",
		Payload: []byte(`{"amount":100}`), SchemaVersion: 1, TenantID: "tenant-1",
		CorrelationID: "corr-1", CausationID: "event-1", OrchestrationOwner: "saga:v1",
		OrchestrationGeneration: 1, TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		TraceState: "vendor=value",
	}
	if err := repo.Enqueue(context.Background(), nil, params); err != nil {
		t.Fatal(err)
	}
	var event Event
	if err := db.First(&event, "message_id = ?", params.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if event.SchemaVersion != 1 || event.TenantID != "tenant-1" || event.CorrelationID != "corr-1" ||
		event.CausationID != "event-1" || event.OrchestrationOwner != "saga:v1" || event.OrchestrationGeneration != 1 ||
		event.TraceParent != params.TraceParent || event.TraceState != params.TraceState {
		t.Fatalf("persisted envelope metadata = %+v", event)
	}
}

func TestMarkRetryPersistsAttemptsAndSchedule(t *testing.T) {
	db := newOutboxSQLiteDB(t)
	repo := NewRepository(db)
	params := EnqueueParams{
		MessageID: "tx-1:settlement", Topic: "settlement.commands", MessageKey: "tx-1",
		MessageType: "CreditCommissionRequested", AggregateType: "PaymentSaga",
		AggregateID: "tx-1", Payload: []byte(`{}`),
	}
	if err := repo.Enqueue(context.Background(), nil, params); err != nil {
		t.Fatal(err)
	}
	var event Event
	if err := db.First(&event, "message_id = ?", params.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Event{}).Where("id = ?", event.ID).Updates(map[string]any{"status": "PROCESSING", "locked_by": "worker-1"}).Error; err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := repo.MarkRetry(context.Background(), event.ID, "worker-1", "broker unavailable", time.Minute, 20); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&event, "id = ?", event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if event.Attempts != 1 || event.Status != "PENDING" || event.ErrorLog != "broker unavailable" || !event.AvailableAt.After(before) {
		t.Fatalf("unexpected retry state: %+v", event)
	}
}

func TestStaleWorkerCannotResurrectProcessedEvent(t *testing.T) {
	db := newOutboxSQLiteDB(t)
	repo := NewRepository(db)
	params := EnqueueParams{
		MessageID: "tx-1:callback", Topic: "agent.commands", MessageKey: "tx-1",
		MessageType: "DispatchCallbackRequested", AggregateType: "PaymentSaga", AggregateID: "tx-1", Payload: []byte(`{}`),
	}
	if err := repo.Enqueue(context.Background(), nil, params); err != nil {
		t.Fatal(err)
	}
	var event Event
	if err := db.First(&event, "message_id = ?", params.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Event{}).Where("id = ?", event.ID).Updates(map[string]any{"status": "PROCESSING", "locked_by": "worker-new"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkProcessed(context.Background(), event.ID, "worker-new"); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkRetry(context.Background(), event.ID, "worker-stale", "late failure", time.Minute, 20); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("MarkRetry() error = %v, want ErrLeaseLost", err)
	}
	if err := db.First(&event, "id = ?", event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if event.Status != "PROCESSED" || event.Attempts != 0 {
		t.Fatalf("stale worker mutated event: %+v", event)
	}
}

func TestClaimUsesSkipLockedAndRecoversStaleProcessingRows(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "message_id", "topic", "message_key", "message_type", "aggregate_type",
		"aggregate_id", "payload", "status", "attempts", "available_at", "locked_at",
		"locked_by", "error_log", "created_at", "processed_at", "dead_at",
	}).AddRow(id, "tx-1:affiliate", "affiliate.commands", "tx-1", "AccumulateVolumeRequested", "PaymentSaga",
		"tx-1", `{}`, "PROCESSING", 1, now, now, "worker-1", "", now, nil, nil)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s).*status = 'PROCESSING'.*locked_at <.*prior.status IN \('PENDING', 'PROCESSING', 'DEAD'\).*FOR UPDATE OF events SKIP LOCKED.*`).
		WithArgs(sqlmock.AnyArg(), 10, "worker-1").
		WillReturnRows(rows)
	mock.ExpectCommit()

	events, err := NewRepository(db).Claim(context.Background(), "worker-1", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != id || events[0].Status != "PROCESSING" {
		t.Fatalf("unexpected claimed events: %+v", events)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func newOutboxSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:outbox-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.outbox_events (
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
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
