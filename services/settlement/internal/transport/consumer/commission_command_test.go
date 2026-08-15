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

type commissionCreditorStub struct {
	calls int
	last  *dto.CalculateAndCreditCommission
}

func (s *commissionCreditorStub) HandleInTx(_ context.Context, _ *gorm.DB, command *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	s.calls++
	copy := *command
	s.last = &copy
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

func TestCommissionCommandPrefersMoneyV2(t *testing.T) {
	db := newInboxDB(t)
	stub := &commissionCreditorStub{}
	handler := &CommissionCommandHandler{db: db, inbox: inbox.NewRepository(), handler: stub, topic: "settlement.commands"}
	payload, _ := json.Marshal(messagebus.CreditCommissionRequested{
		TransactionID: "tx-money", AgentID: "agent-1", Rank: "SILVER",
		TransactionMoney:  &messagebus.Money{AmountMinor: 10_001, Currency: "USD", Exponent: 2},
		TransactionAmount: 999.99, Currency: "USD",
	})
	if err := handler.Handle(context.Background(), &kafka.Message{ID: "message-money", EventType: messagebus.CreditCommissionRequestedType, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if stub.last == nil || stub.last.TransactionMoney == nil || stub.last.TransactionMoney.AmountMinor != 10_001 {
		t.Fatalf("forwarded command = %+v", stub.last)
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
			consumer_name TEXT NOT NULL, message_id TEXT NOT NULL, topic TEXT NOT NULL, message_type TEXT NOT NULL,
			status TEXT NOT NULL, created_at DATETIME, processed_at DATETIME,
			PRIMARY KEY (consumer_name, message_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
