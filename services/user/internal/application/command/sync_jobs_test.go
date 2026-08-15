package command

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"gorm.io/gorm"
)

type syncTransactionMarker struct{}

type syncUnitOfWorkStub struct{ calls int }

func (u *syncUnitOfWorkStub) Do(ctx context.Context, fn func(context.Context) error) error {
	u.calls++
	return fn(context.WithValue(ctx, syncTransactionMarker{}, true))
}

type syncUserRepositoryStub struct {
	users          []domain.UserIdentity
	outsideTxWrite bool
}

func (r *syncUserRepositoryStub) Upsert(ctx context.Context, user *domain.UserIdentity) error {
	if ctx.Value(syncTransactionMarker{}) != true {
		r.outsideTxWrite = true
	}
	if user.ID == "" {
		user.ID = "local-" + user.IdentityProviderUserID
	}
	r.users = append(r.users, *user)
	return nil
}

func (*syncUserRepositoryStub) GetByID(context.Context, string) (*domain.UserIdentity, error) {
	return nil, nil
}
func (*syncUserRepositoryStub) GetByProviderUserID(context.Context, string, string) (*domain.UserIdentity, error) {
	return nil, nil
}
func (*syncUserRepositoryStub) GetByUsername(context.Context, string, string) (*domain.UserIdentity, error) {
	return nil, nil
}
func (*syncUserRepositoryStub) ListAll(context.Context, int, int) ([]*domain.UserIdentity, int64, error) {
	return nil, 0, nil
}

type syncEventStoreStub struct {
	events         []*eventstore.DomainEvent
	outsideTxWrite bool
}

func (s *syncEventStoreStub) AppendEvent(ctx context.Context, _ *gorm.DB, event *eventstore.DomainEvent) error {
	if ctx.Value(syncTransactionMarker{}) != true {
		s.outsideTxWrite = true
	}
	s.events = append(s.events, event)
	return nil
}
func (*syncEventStoreStub) LoadEvents(context.Context, string, string) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}
func (*syncEventStoreStub) LoadEventsFrom(context.Context, string, string, int64) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}
func (s *syncEventStoreStub) NextVersion(ctx context.Context, _ *gorm.DB, _, _ string) (int64, error) {
	if ctx.Value(syncTransactionMarker{}) != true {
		s.outsideTxWrite = true
	}
	return int64(len(s.events) + 1), nil
}

type syncGatewayStub struct {
	user              *domain.UserIdentity
	page              port.UserSyncPage
	err               error
	calledInsideTx    bool
	requestedCursor   int
	requestedPageSize int
}

func (g *syncGatewayStub) FetchAllUsers(ctx context.Context) ([]domain.UserIdentity, error) {
	if ctx.Value(syncTransactionMarker{}) == true {
		g.calledInsideTx = true
	}
	return g.page.Users, g.err
}

func (g *syncGatewayStub) FetchUsersPage(ctx context.Context, cursor, limit int) (port.UserSyncPage, error) {
	if ctx.Value(syncTransactionMarker{}) == true {
		g.calledInsideTx = true
	}
	g.requestedCursor = cursor
	g.requestedPageSize = limit
	return g.page, g.err
}

func (g *syncGatewayStub) FetchUserByID(ctx context.Context, _ string) (*domain.UserIdentity, error) {
	if ctx.Value(syncTransactionMarker{}) == true {
		g.calledInsideTx = true
	}
	return g.user, g.err
}

type syncJobRepositoryStub struct {
	claimed          *domain.UserSyncJob
	enqueued         []*domain.UserSyncJob
	advanced         bool
	advanceInsideTx  bool
	nextCursor       int
	processedDelta   int
	completed        bool
	retried          bool
	retryDelay       time.Duration
	markRetryFailure error
}

func (r *syncJobRepositoryStub) Enqueue(_ context.Context, job *domain.UserSyncJob) (bool, error) {
	copy := *job
	copy.ID = fmt.Sprintf("job-%d", len(r.enqueued)+1)
	copy.Status = domain.UserSyncJobPending
	*job = copy
	r.enqueued = append(r.enqueued, &copy)
	return true, nil
}

func (r *syncJobRepositoryStub) ClaimNext(context.Context, string, time.Duration) (*domain.UserSyncJob, error) {
	job := r.claimed
	r.claimed = nil
	return job, nil
}

func (r *syncJobRepositoryStub) Advance(
	ctx context.Context,
	_, _ string,
	_ int64,
	nextCursor, processedDelta int,
	completed bool,
) error {
	r.advanced = true
	r.advanceInsideTx = ctx.Value(syncTransactionMarker{}) == true
	r.nextCursor = nextCursor
	r.processedDelta = processedDelta
	r.completed = completed
	return nil
}

func (r *syncJobRepositoryStub) MarkRetry(
	_ context.Context,
	_, _ string,
	_ int64,
	_ string,
	delay time.Duration,
	_ int,
) error {
	r.retried = true
	r.retryDelay = delay
	return r.markRetryFailure
}

func testSyncJobPolicy() UserSyncJobPolicy {
	return UserSyncJobPolicy{
		PageSize: 100, LeaseDuration: time.Minute, ProviderTimeout: 10 * time.Second,
		MaxAttempts: 5, RetryBaseDelay: time.Second, RetryMaxDelay: time.Minute,
	}
}

func newSyncJobRunnerForTest(
	t *testing.T,
	jobs *syncJobRepositoryStub,
	gateway *syncGatewayStub,
) (*UserSyncJobRunner, *syncUnitOfWorkStub, *syncUserRepositoryStub, *syncEventStoreStub) {
	t.Helper()
	uow := new(syncUnitOfWorkStub)
	users := new(syncUserRepositoryStub)
	events := new(syncEventStoreStub)
	syncUsers := NewSyncUsersHandler(uow, users, gateway, events)
	runner, err := NewUserSyncJobRunner(jobs, gateway, syncUsers, testSyncJobPolicy())
	if err != nil {
		t.Fatal(err)
	}
	return runner, uow, users, events
}

func TestSingleUserJobCallsProviderOutsideTransactionAndCompletesAtomically(t *testing.T) {
	jobs := &syncJobRepositoryStub{claimed: &domain.UserSyncJob{
		ID: "job-1", Kind: domain.UserSyncJobSingle, ProviderUserID: "provider-1",
		PageSize: 1, Status: domain.UserSyncJobProcessing, LeaseToken: 7,
	}}
	gateway := &syncGatewayStub{user: &domain.UserIdentity{
		IdentityProvider: "keycloak", IdentityProviderUserID: "provider-1", Username: "user-1",
	}}
	runner, uow, users, events := newSyncJobRunnerForTest(t, jobs, gateway)

	claimed, err := runner.RunNext(context.Background(), "worker-1")
	if err != nil || !claimed {
		t.Fatalf("RunNext() claimed=%v error=%v", claimed, err)
	}
	if gateway.calledInsideTx {
		t.Fatal("identity provider was called inside the UnitOfWork transaction")
	}
	if uow.calls != 1 || users.outsideTxWrite || events.outsideTxWrite || !jobs.advanceInsideTx {
		t.Fatalf("transaction boundaries: uow=%d userOutside=%v eventOutside=%v advanceInside=%v",
			uow.calls, users.outsideTxWrite, events.outsideTxWrite, jobs.advanceInsideTx)
	}
	if len(users.users) != 1 || len(events.events) != 1 || !jobs.completed || jobs.processedDelta != 1 {
		t.Fatalf("sync result: users=%d events=%d completed=%v delta=%d",
			len(users.users), len(events.events), jobs.completed, jobs.processedDelta)
	}
}

func TestFullUserJobPersistsOnePageAndResumableCursor(t *testing.T) {
	jobs := &syncJobRepositoryStub{claimed: &domain.UserSyncJob{
		ID: "job-full", Kind: domain.UserSyncJobFull, Cursor: 100, PageSize: 2,
		Status: domain.UserSyncJobProcessing, LeaseToken: 3,
	}}
	gateway := &syncGatewayStub{page: port.UserSyncPage{
		Users: []domain.UserIdentity{
			{IdentityProvider: "keycloak", IdentityProviderUserID: "provider-101", Username: "user-101"},
			{IdentityProvider: "keycloak", IdentityProviderUserID: "provider-102", Username: "user-102"},
		},
		NextCursor: 102, Done: false,
	}}
	runner, _, users, _ := newSyncJobRunnerForTest(t, jobs, gateway)

	claimed, err := runner.RunNext(context.Background(), "worker-1")
	if err != nil || !claimed {
		t.Fatalf("RunNext() claimed=%v error=%v", claimed, err)
	}
	if gateway.requestedCursor != 100 || gateway.requestedPageSize != 2 {
		t.Fatalf("provider page request cursor=%d size=%d", gateway.requestedCursor, gateway.requestedPageSize)
	}
	if len(users.users) != 2 || jobs.nextCursor != 102 || jobs.processedDelta != 2 || jobs.completed {
		t.Fatalf("page result users=%d cursor=%d delta=%d completed=%v",
			len(users.users), jobs.nextCursor, jobs.processedDelta, jobs.completed)
	}
}

func TestProviderFailureIsRescheduledWithoutOpeningTransaction(t *testing.T) {
	jobs := &syncJobRepositoryStub{claimed: &domain.UserSyncJob{
		ID: "job-1", Kind: domain.UserSyncJobSingle, ProviderUserID: "provider-1",
		PageSize: 1, Status: domain.UserSyncJobProcessing, Attempts: 2, LeaseToken: 4,
	}}
	gateway := &syncGatewayStub{err: errors.New("keycloak unavailable")}
	runner, uow, _, _ := newSyncJobRunnerForTest(t, jobs, gateway)

	claimed, err := runner.RunNext(context.Background(), "worker-1")
	if !claimed || err == nil {
		t.Fatalf("RunNext() claimed=%v error=%v", claimed, err)
	}
	if uow.calls != 0 || !jobs.retried || jobs.retryDelay != 4*time.Second {
		t.Fatalf("failure handling uow=%d retried=%v delay=%s", uow.calls, jobs.retried, jobs.retryDelay)
	}
}

func TestRequestUserSyncBuildsDeterministicRegistrationIntent(t *testing.T) {
	jobs := new(syncJobRepositoryStub)
	handler, err := NewRequestUserSyncHandler(jobs, testSyncJobPolicy())
	if err != nil {
		t.Fatal(err)
	}
	job, created, err := handler.EnqueueRegistered(context.Background(), "event-1", "provider-1")
	if err != nil || !created {
		t.Fatalf("EnqueueRegistered() created=%v error=%v", created, err)
	}
	if job.DeduplicationKey != "user-registered:event-1" || job.Kind != domain.UserSyncJobSingle || job.PageSize != 1 {
		t.Fatalf("unexpected job: %+v", job)
	}
}
