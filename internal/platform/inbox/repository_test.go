package inbox

import (
	"context"
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMessageIdentityIsScopedByLogicalConsumer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inbox-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.inbox_messages (
		consumer_name TEXT NOT NULL, message_id TEXT NOT NULL, topic TEXT NOT NULL,
		message_type TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME,
		processed_at DATETIME, PRIMARY KEY (consumer_name, message_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository()
	ctx := context.Background()
	first, err := repo.Begin(ctx, db, "affiliate-service", "message-1", "payments.events", "PaymentCompleted")
	if err != nil || !first {
		t.Fatalf("first delivery: inserted=%v error=%v", first, err)
	}
	duplicate, err := repo.Begin(ctx, db, "affiliate-service", "message-1", "payments.events", "PaymentCompleted")
	if err != nil || duplicate {
		t.Fatalf("duplicate delivery: inserted=%v error=%v", duplicate, err)
	}
	independent, err := repo.Begin(ctx, db, "settlement-service", "message-1", "payments.events", "PaymentCompleted")
	if err != nil || !independent {
		t.Fatalf("independent consumer: inserted=%v error=%v", independent, err)
	}
}
