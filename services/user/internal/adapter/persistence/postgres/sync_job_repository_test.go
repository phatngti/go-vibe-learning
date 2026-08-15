package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/user/internal/application/command"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSyncJobEnqueueIsIdempotentAndHonorsTransactionContext(t *testing.T) {
	db := newUserSyncJobSQLiteDB(t)
	repository := &userSyncJobRepository{db: db}
	job := &domain.UserSyncJob{
		DeduplicationKey: "user-registered:event-1", Kind: domain.UserSyncJobSingle,
		IdentityProvider: "keycloak", ProviderUserID: "provider-1", PageSize: 1,
	}

	tx := db.Begin()
	created, err := repository.Enqueue(platformpostgres.WithTx(context.Background(), tx), job)
	if err != nil || !created {
		t.Fatalf("Enqueue() created=%v error=%v", created, err)
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&userSyncJobRow{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back intent count = %d, want 0", count)
	}

	created, err = repository.Enqueue(context.Background(), job)
	if err != nil || !created {
		t.Fatalf("committed Enqueue() created=%v error=%v", created, err)
	}
	firstID := job.ID
	replay := &domain.UserSyncJob{
		DeduplicationKey: job.DeduplicationKey, Kind: job.Kind,
		IdentityProvider: job.IdentityProvider, ProviderUserID: job.ProviderUserID, PageSize: job.PageSize,
	}
	created, err = repository.Enqueue(context.Background(), replay)
	if err != nil || created || replay.ID != firstID {
		t.Fatalf("replay Enqueue() created=%v id=%q error=%v", created, replay.ID, err)
	}
}

func TestSyncJobAdvanceRejectsStaleFencingToken(t *testing.T) {
	db := newUserSyncJobSQLiteDB(t)
	repository := &userSyncJobRepository{db: db}
	job := &domain.UserSyncJob{
		DeduplicationKey: "full-sync:request-1", Kind: domain.UserSyncJobFull,
		IdentityProvider: "keycloak", PageSize: 100,
	}
	if _, err := repository.Enqueue(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&userSyncJobRow{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status": domain.UserSyncJobProcessing, "lease_owner": "worker-new", "lease_token": 2,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := repository.Advance(context.Background(), job.ID, "worker-old", 1, 100, 100, false); !errors.Is(err, port.ErrUserSyncJobLeaseLost) {
		t.Fatalf("stale Advance() error = %v", err)
	}
	if err := repository.Advance(context.Background(), job.ID, "worker-new", 2, 100, 100, false); err != nil {
		t.Fatal(err)
	}
	var row userSyncJobRow
	if err := db.First(&row, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != string(domain.UserSyncJobPending) || row.Cursor != 100 || row.ProcessedCount != 100 || row.LeaseOwner != "" {
		t.Fatalf("advanced row = %+v", row)
	}
}

func TestStaleLeaseRollsBackUserPageAndDomainEvents(t *testing.T) {
	db := newUserSyncJobSQLiteDB(t)
	jobs := &userSyncJobRepository{db: db}
	job := &domain.UserSyncJob{
		DeduplicationKey: "full-sync:request-rollback", Kind: domain.UserSyncJobFull,
		IdentityProvider: "keycloak", PageSize: 100,
	}
	if _, err := jobs.Enqueue(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&userSyncJobRow{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status": domain.UserSyncJobProcessing, "lease_owner": "worker-new", "lease_token": 2,
	}).Error; err != nil {
		t.Fatal(err)
	}

	users := NewUserRepository(&platformpostgres.DB{DB: db})
	uow := NewUnitOfWork(&platformpostgres.DB{DB: db})
	handler := command.NewSyncUsersHandler(uow, users, nil, eventstore.NewRepository(db))
	err := handler.SyncBatch(context.Background(), []domain.UserIdentity{{
		ID: "local-1", IdentityProvider: "keycloak", IdentityProviderUserID: "provider-1", Username: "user-1",
	}}, func(txCtx context.Context) error {
		return jobs.Advance(txCtx, job.ID, "worker-old", 1, 1, 1, true)
	})
	if !errors.Is(err, port.ErrUserSyncJobLeaseLost) {
		t.Fatalf("SyncBatch() error = %v, want lease lost", err)
	}
	var userCount, eventCount int64
	if err := db.Table("public.user_identities").Count(&userCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("public.domain_events").Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if userCount != 0 || eventCount != 0 {
		t.Fatalf("stale worker committed users=%d events=%d", userCount, eventCount)
	}
}

func TestSyncJobRetryIsBoundedAndTransitionsToDead(t *testing.T) {
	db := newUserSyncJobSQLiteDB(t)
	repository := &userSyncJobRepository{db: db}
	job := &domain.UserSyncJob{
		DeduplicationKey: "user-registered:event-dead", Kind: domain.UserSyncJobSingle,
		IdentityProvider: "keycloak", ProviderUserID: "provider-dead", PageSize: 1,
	}
	if _, err := repository.Enqueue(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&userSyncJobRow{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status": domain.UserSyncJobProcessing, "lease_owner": "worker-1", "lease_token": 5, "attempts": 11,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkRetry(
		context.Background(), job.ID, "worker-1", 5, "permanent outage", time.Minute, 12,
	); err != nil {
		t.Fatal(err)
	}
	var row userSyncJobRow
	if err := db.First(&row, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != string(domain.UserSyncJobDead) || row.Attempts != 12 || row.DeadAt == nil || row.LeaseOwner != "" {
		t.Fatalf("dead job row = %+v", row)
	}
	if err := repository.MarkRetry(
		context.Background(), job.ID, "worker-1", 5, "late retry", 0, 12,
	); !errors.Is(err, port.ErrUserSyncJobLeaseLost) {
		t.Fatalf("late MarkRetry() error = %v", err)
	}
}

func TestSyncJobClaimUsesSkipLockedAndFencesExpiredLease(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	rows := sqlmock.NewRows([]string{
		"id", "deduplication_key", "kind", "identity_provider", "provider_user_id",
		"cursor", "page_size", "processed_count", "status", "attempts", "available_at",
		"lease_owner", "lease_token", "lease_expires_at", "last_error", "created_at",
		"updated_at", "completed_at", "dead_at",
	}).AddRow(
		id, "full-sync:request-1", "FULL", "keycloak", "", 100, 100, 100,
		"PROCESSING", 1, now, "worker-2", 8, now.Add(time.Minute), "", now, now, nil, nil,
	)
	mock.ExpectQuery(`(?s).*status = 'PROCESSING'.*lease_expires_at <.*FOR UPDATE OF jobs SKIP LOCKED.*lease_token = jobs.lease_token \+ 1.*`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "worker-2", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)

	job, err := repositoryFromGorm(db).ClaimNext(context.Background(), "worker-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.ID != id || job.LeaseToken != 8 || job.Cursor != 100 {
		t.Fatalf("claimed job = %+v", job)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func repositoryFromGorm(db *gorm.DB) *userSyncJobRepository {
	return &userSyncJobRepository{db: db}
}

func newUserSyncJobSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:user-sync-jobs-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ATTACH DATABASE ':memory:' AS public").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.user_sync_jobs (
		id TEXT PRIMARY KEY, deduplication_key TEXT NOT NULL UNIQUE, kind TEXT NOT NULL,
		identity_provider TEXT NOT NULL, provider_user_id TEXT NOT NULL DEFAULT '', cursor INTEGER NOT NULL,
		page_size INTEGER NOT NULL, processed_count INTEGER NOT NULL, status TEXT NOT NULL, attempts INTEGER NOT NULL,
		available_at DATETIME NOT NULL, lease_owner TEXT NOT NULL, lease_token INTEGER NOT NULL,
		lease_expires_at DATETIME, last_error TEXT NOT NULL, created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL, completed_at DATETIME, dead_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.user_identities (
		id TEXT PRIMARY KEY, identity_provider TEXT NOT NULL, identity_provider_user_id TEXT NOT NULL,
		username TEXT NOT NULL, email TEXT, first_name TEXT, last_name TEXT, email_verified BOOLEAN NOT NULL,
		enabled BOOLEAN NOT NULL, attributes TEXT NOT NULL, identity_created_at DATETIME,
		last_synced_at DATETIME NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		UNIQUE (identity_provider, identity_provider_user_id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE public.domain_events (
		id TEXT PRIMARY KEY, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL,
		event_type TEXT NOT NULL, payload TEXT NOT NULL, version INTEGER NOT NULL,
		occurred_at DATETIME NOT NULL, UNIQUE (aggregate_type, aggregate_id, version)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
