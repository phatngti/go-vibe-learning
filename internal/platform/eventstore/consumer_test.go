package eventstore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConsumerRetriesWithoutAdvancingCheckpoint(t *testing.T) {
	db := newConsumerTestDB(t)
	events := []*DomainEvent{
		{ID: uuid.New(), AggregateType: "Transaction", AggregateID: "tx-1", EventType: "PaymentInitiated", Payload: `{}`, Version: 1, OccurredAt: time.Now().Add(-time.Second)},
		{ID: uuid.New(), AggregateType: "Transaction", AggregateID: "tx-1", EventType: "PaymentCompleted", Payload: `{}`, Version: 2, OccurredAt: time.Now()},
	}
	for _, event := range events {
		if err := db.Create(event).Error; err != nil {
			t.Fatal(err)
		}
	}
	consumer := &Consumer{db: db}

	processed, err := consumer.ConsumeBatch(context.Background(), "payment", "Transaction", nil, 10,
		func(context.Context, *gorm.DB, *DomainEvent) error { return errors.New("temporary failure") })
	if err == nil || processed != 0 {
		t.Fatalf("first batch processed = %d, error = %v", processed, err)
	}

	var handled []string
	processed, err = consumer.ConsumeBatch(context.Background(), "payment", "Transaction", nil, 10,
		func(_ context.Context, _ *gorm.DB, event *DomainEvent) error {
			handled = append(handled, event.EventType)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if processed != 2 || len(handled) != 2 {
		t.Fatalf("processed = %d, handled = %v", processed, handled)
	}

	processed, err = consumer.ConsumeBatch(context.Background(), "payment", "Transaction", nil, 10,
		func(context.Context, *gorm.DB, *DomainEvent) error { return nil })
	if err != nil || processed != 0 {
		t.Fatalf("replay batch processed = %d, error = %v", processed, err)
	}
}

func newConsumerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:event-consumer-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE public.domain_events (
			id TEXT PRIMARY KEY, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL,
			event_type TEXT NOT NULL, payload TEXT NOT NULL, version INTEGER NOT NULL,
			occurred_at DATETIME NOT NULL
		)`,
		`CREATE TABLE public.event_consumers (
			consumer_name TEXT PRIMARY KEY, last_event_time DATETIME NOT NULL,
			last_event_id TEXT NOT NULL, updated_at DATETIME NOT NULL
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}
