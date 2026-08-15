package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/observability/logger"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/user/internal/application/command"
	"github/master_go_cqrs/services/user/internal/domain"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type eventIntentRepositoryStub struct {
	transaction *gorm.DB
	job         *domain.UserSyncJob
}

func (r *eventIntentRepositoryStub) Enqueue(ctx context.Context, job *domain.UserSyncJob) (bool, error) {
	r.transaction = platformpostgres.FromContext(ctx, nil)
	copy := *job
	copy.ID = "job-1"
	*job = copy
	r.job = &copy
	return true, nil
}

func (*eventIntentRepositoryStub) ClaimNext(context.Context, string, time.Duration) (*domain.UserSyncJob, error) {
	return nil, nil
}
func (*eventIntentRepositoryStub) Advance(context.Context, string, string, int64, int, int, bool) error {
	return nil
}
func (*eventIntentRepositoryStub) MarkRetry(context.Context, string, string, int64, string, time.Duration, int) error {
	return nil
}

func TestUserRegisteredHandlerPersistsIntentInCheckpointTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-registered-handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	repository := new(eventIntentRepositoryStub)
	policy := command.UserSyncJobPolicy{
		PageSize: 100, LeaseDuration: time.Minute, ProviderTimeout: 10 * time.Second,
		MaxAttempts: 5, RetryBaseDelay: time.Second, RetryMaxDelay: time.Minute,
	}
	requestSync, err := command.NewRequestUserSyncHandler(repository, policy)
	if err != nil {
		t.Fatal(err)
	}
	logFactory := logger.NewFactory(logger.NewapiLogger(&config.Logger{
		LogLevel: "error", Mode: "production", LogEncoding: "json",
	}))
	handler := NewUserRegisteredHandler(requestSync, logFactory)
	payload, err := json.Marshal(domain.UserRegistered{IdentityProviderUserID: "provider-1"})
	if err != nil {
		t.Fatal(err)
	}
	event := &eventstore.DomainEvent{ID: uuid.New(), Payload: string(payload)}

	if err := handler.Handle(context.Background(), db, event); err != nil {
		t.Fatal(err)
	}
	if repository.transaction != db {
		t.Fatal("sync intent did not use the event checkpoint transaction")
	}
	if repository.job == nil || repository.job.ProviderUserID != "provider-1" ||
		repository.job.DeduplicationKey != "user-registered:"+event.ID.String() {
		t.Fatalf("unexpected persisted job: %+v", repository.job)
	}
}
