package outbox

import (
	"context"
	"testing"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/kafka"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPublisherPropagatesFactVersionAndEnvelopeMetadata(t *testing.T) {
	eventID := uuid.New()
	repository := &publisherRepositoryStub{events: []*Event{{
		ID: eventID, MessageID: "user-1:user-synced:v7", Topic: "users.events.v2", MessageKey: "user-1",
		MessageType: "UserSynced", AggregateType: "UserIdentity", AggregateID: "user-1", Payload: `{}`,
		SchemaVersion: 2, TenantID: "tenant-1", CorrelationID: "correlation-1", CausationID: "command-1",
		OrchestrationOwner: "temporal:v2", OrchestrationGeneration: 2,
		TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", TraceState: "vendor=value",
		CreatedAt: time.Now(),
	}}}
	producer := &publisherProducerStub{}
	publisher := &Publisher{
		repo: repository, producer: producer, workerID: "worker-1",
		config: platformconfig.Outbox{BatchSize: 1, Parallelism: 1, MaxAttempts: 3},
	}

	published, err := publisher.publishBatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if published != 1 || repository.processed != eventID || producer.message == nil {
		t.Fatalf("published = %d, processed = %s, message = %+v", published, repository.processed, producer.message)
	}
	message := producer.message
	if message.AggregateVersion != 7 || message.SchemaVersion != 2 || message.TenantID != "tenant-1" ||
		message.CorrelationID != "correlation-1" || message.CausationID != "command-1" ||
		message.OrchestrationOwner != "temporal:v2" || message.OrchestrationGeneration != 2 ||
		message.TraceParent == "" || message.TraceState != "vendor=value" {
		t.Fatalf("published envelope = %+v", message)
	}
}

type publisherRepositoryStub struct {
	events    []*Event
	processed uuid.UUID
}

func (*publisherRepositoryStub) Enqueue(context.Context, *gorm.DB, EnqueueParams) error { return nil }

func (r *publisherRepositoryStub) Claim(context.Context, string, int, time.Duration) ([]*Event, error) {
	return r.events, nil
}

func (r *publisherRepositoryStub) MarkProcessed(_ context.Context, id uuid.UUID, _ string) error {
	r.processed = id
	return nil
}

func (*publisherRepositoryStub) MarkRetry(context.Context, uuid.UUID, string, string, time.Duration, int) error {
	return nil
}

func (*publisherRepositoryStub) Stats(context.Context) (Stats, error) { return Stats{}, nil }

type publisherProducerStub struct {
	message *kafka.Message
}

func (p *publisherProducerStub) Publish(_ context.Context, _, _ string, message *kafka.Message) error {
	copy := *message
	p.message = &copy
	return nil
}

func (*publisherProducerStub) Close() error { return nil }
