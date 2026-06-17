package saga

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHandlerDeduplicatesEventsBeforeCallingProcessor(t *testing.T) {
	db := newCoreTestDB(t)
	processor := &eventProcessorStub{}
	handler := NewHandler(db, inbox.NewRepository(), processor)
	message := &kafka.Message{ID: "message-1", EventType: "process.started", Payload: []byte(`{}`)}

	if err := handler.Handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}

	if processor.calls != 1 {
		t.Fatalf("processor calls = %d, want 1", processor.calls)
	}
}

func TestReconcilerDelegatesDueInstancesToRetryProcessor(t *testing.T) {
	processor := &retryProcessorStub{due: []string{"instance-1", "instance-2"}}
	reconciler := NewReconciler(processor, logger.Factory{}, ReconcilerConfig{BatchSize: 25})

	reconciler.Reconcile(context.Background())

	if processor.limit != 25 {
		t.Fatalf("due limit = %d, want 25", processor.limit)
	}
	if len(processor.retried) != 2 || processor.retried[0] != "instance-1" || processor.retried[1] != "instance-2" {
		t.Fatalf("retried instances = %v", processor.retried)
	}
}

type eventProcessorStub struct {
	calls int
}

func (*eventProcessorStub) TopicFor(string) string { return "process.events" }

func (p *eventProcessorStub) ProcessEvent(context.Context, *gorm.DB, *kafka.Message) error {
	p.calls++
	return nil
}

type retryProcessorStub struct {
	due     []string
	limit   int
	retried []string
}

func (*retryProcessorStub) Name() string { return "test" }

func (p *retryProcessorStub) Due(_ context.Context, limit int) ([]string, error) {
	p.limit = limit
	return p.due, nil
}

func (p *retryProcessorStub) Retry(_ context.Context, instanceID string, _ time.Time) error {
	p.retried = append(p.retried, instanceID)
	return nil
}

func newCoreTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:saga-core-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.inbox_messages (
		message_id TEXT PRIMARY KEY, topic TEXT NOT NULL, message_type TEXT NOT NULL,
		status TEXT NOT NULL, created_at DATETIME, processed_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
