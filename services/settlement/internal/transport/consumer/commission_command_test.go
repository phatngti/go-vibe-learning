package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/settlement/internal/application/dto"
	"github/master_go_cqrs/services/settlement/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type commissionCreditorStub struct{ calls int }

func (s *commissionCreditorStub) HandleInTx(context.Context, *gorm.DB, *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	s.calls++
	return &domain.AgentBalance{}, 1, nil
}

func TestCommissionCommandDeduplicatesByTransactionID(t *testing.T) {
	db := newInboxDB(t)
	stub := &commissionCreditorStub{}
	handler := &CommissionCommandHandler{db: db, inbox: inbox.NewRepository(), handler: stub, topic: "settlement.commands"}
	payload, _ := json.Marshal(messagebus.CreditCommissionRequested{
		TransactionID: "tx-1", AgentID: "agent-1", TransactionAmount: 100, Rank: "BRONZE",
	})

	for _, id := range []string{"message-1", "message-2"} {
		if err := handler.Handle(context.Background(), &kafka.Message{ID: id, EventType: messagebus.CreditCommissionRequestedType, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	if stub.calls != 1 {
		t.Fatalf("handler calls = %d, want 1", stub.calls)
	}
}

func newInboxDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
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
