package eventstore

import (
	"context"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/messageidentity"
	"github/master_go_cqrs/internal/platform/outbox"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestRelayUsesAggregateVersionForRecurringFactIdentity(t *testing.T) {
	outboxRepo := &recordingOutboxRepository{}
	relay := NewRelayRepository(recordingEventRepository{}, outboxRepo, RelayConfig{
		"UserSynced": {Topic: "user.events", MessageType: "UserSynced", StepName: "user-synced"},
	})

	for version := int64(1); version <= 2; version++ {
		event := &DomainEvent{
			AggregateType: "UserIdentity", AggregateID: "user-1", EventType: "UserSynced",
			Payload: `{"user_id":"user-1"}`, Version: version,
		}
		if err := relay.AppendEvent(context.Background(), nil, event); err != nil {
			t.Fatalf("AppendEvent(version %d) error = %v", version, err)
		}
	}

	if len(outboxRepo.enqueued) != 2 {
		t.Fatalf("enqueued facts = %d, want 2", len(outboxRepo.enqueued))
	}
	firstID := outboxRepo.enqueued[0].MessageID
	secondID := outboxRepo.enqueued[1].MessageID
	if firstID != messageidentity.Fact("user-1", "user-synced", 1) {
		t.Fatalf("first fact ID = %q", firstID)
	}
	if secondID != messageidentity.Fact("user-1", "user-synced", 2) {
		t.Fatalf("second fact ID = %q", secondID)
	}
	if firstID == secondID {
		t.Fatalf("recurring facts share identity %q", firstID)
	}
}

type recordingEventRepository struct{}

func (recordingEventRepository) AppendEvent(context.Context, *gorm.DB, *DomainEvent) error {
	return nil
}
func (recordingEventRepository) LoadEvents(context.Context, string, string) ([]*DomainEvent, error) {
	return nil, nil
}
func (recordingEventRepository) LoadEventsFrom(context.Context, string, string, int64) ([]*DomainEvent, error) {
	return nil, nil
}
func (recordingEventRepository) NextVersion(context.Context, *gorm.DB, string, string) (int64, error) {
	return 1, nil
}

type recordingOutboxRepository struct {
	enqueued []outbox.EnqueueParams
}

func (r *recordingOutboxRepository) Enqueue(_ context.Context, _ *gorm.DB, params outbox.EnqueueParams) error {
	r.enqueued = append(r.enqueued, params)
	return nil
}
func (*recordingOutboxRepository) Claim(context.Context, string, int, time.Duration) ([]*outbox.Event, error) {
	return nil, nil
}
func (*recordingOutboxRepository) MarkProcessed(context.Context, uuid.UUID, string) error {
	return nil
}
func (*recordingOutboxRepository) MarkRetry(context.Context, uuid.UUID, string, string, time.Duration, int) error {
	return nil
}
func (*recordingOutboxRepository) Stats(context.Context) (outbox.Stats, error) {
	return outbox.Stats{}, nil
}
