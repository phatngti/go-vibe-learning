// File: internal/platform/outbox/repository_test.go
// Description: Unit and mock tests for outbox repository operations.

package outbox

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnqueueIsIdempotentAndClearsProcessedState(t *testing.T) {
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
	if err := repo.MarkProcessed(context.Background(), first.ID); err != nil {
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
	if len(events) != 1 || events[0].Status != "PENDING" || events[0].ProcessedAt != nil || events[0].Payload != string(params.Payload) {
		t.Fatalf("unexpected re-enqueued event: %+v", events)
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
	before := time.Now()
	if err := repo.MarkRetry(context.Background(), event.ID, "broker unavailable", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&event, "id = ?", event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if event.Attempts != 1 || event.Status != "PENDING" || event.ErrorLog != "broker unavailable" || !event.AvailableAt.After(before) {
		t.Fatalf("unexpected retry state: %+v", event)
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
		"locked_by", "error_log", "created_at", "processed_at",
	}).AddRow(id, "tx-1:affiliate", "affiliate.commands", "tx-1", "AccumulateVolumeRequested", "PaymentSaga",
		"tx-1", `{}`, "PROCESSING", 1, now, now, "worker-1", "", now, nil)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s).*status = 'PROCESSING'.*locked_at <.*FOR UPDATE SKIP LOCKED.*`).
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
		attempts INTEGER NOT NULL DEFAULT 0, available_at DATETIME, locked_at DATETIME,
		locked_by TEXT, error_log TEXT, created_at DATETIME, processed_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
