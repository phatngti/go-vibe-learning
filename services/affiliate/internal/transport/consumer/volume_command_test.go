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

type volumeAccumulatorStub struct{ calls int }

func (s *volumeAccumulatorStub) HandleInTx(context.Context, *gorm.DB, *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	s.calls++
	return &domain.AgentMetric{}, domain.RankBronze, false, nil
}

func TestVolumeCommandDeduplicatesByTransactionID(t *testing.T) {
	db := newInboxDB(t)
	stub := &volumeAccumulatorStub{}
	handler := &VolumeCommandHandler{db: db, inbox: inbox.NewRepository(), handler: stub, topic: "affiliate.commands"}
	payload, _ := json.Marshal(messagebus.AccumulateVolumeRequested{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100})

	for _, id := range []string{"message-1", "message-2"} {
		if err := handler.Handle(context.Background(), &kafka.Message{ID: id, EventType: messagebus.AccumulateVolumeRequestedType, Payload: payload}); err != nil {
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
