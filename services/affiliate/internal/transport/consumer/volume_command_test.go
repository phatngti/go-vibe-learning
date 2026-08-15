package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/affiliate/internal/application/dto"
	"github/master_go_cqrs/services/affiliate/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type volumeAccumulatorStub struct {
	calls int
	last  *dto.AccumulateVolume
}

func (s *volumeAccumulatorStub) HandleInTx(_ context.Context, _ *gorm.DB, command *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	s.calls++
	copy := *command
	s.last = &copy
	return &domain.AgentMetric{}, domain.RankBronze, false, nil
}

func TestVolumeCommandDeduplicatesByTransactionID(t *testing.T) {
	db := newInboxDB(t)
	stub := &volumeAccumulatorStub{}
	handler := &VolumeCommandHandler{db: db, inbox: inbox.NewRepository(), handler: stub, topic: "affiliate.commands"}
	payload, _ := json.Marshal(messagebus.AccumulateVolumeRequested{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD"})

	for _, id := range []string{"message-1", "message-2"} {
		if err := handler.Handle(context.Background(), &kafka.Message{ID: id, EventType: messagebus.AccumulateVolumeRequestedType, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	if stub.calls != 1 {
		t.Fatalf("handler calls = %d, want 1", stub.calls)
	}
}

func TestVolumeCommandPrefersMoneyV2(t *testing.T) {
	db := newInboxDB(t)
	stub := &volumeAccumulatorStub{}
	handler := &VolumeCommandHandler{db: db, inbox: inbox.NewRepository(), handler: stub, topic: "affiliate.commands"}
	payload, _ := json.Marshal(messagebus.AccumulateVolumeRequested{
		TransactionID: "tx-money", AgentID: "agent-1",
		Money:  &messagebus.Money{AmountMinor: 12_345, Currency: "USD", Exponent: 2},
		Amount: 999.99, Currency: "USD",
	})
	if err := handler.Handle(context.Background(), &kafka.Message{ID: "message-money", EventType: messagebus.AccumulateVolumeRequestedType, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if stub.last == nil || stub.last.Money == nil || stub.last.Money.AmountMinor != 12_345 {
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
