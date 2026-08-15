package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userSyncJobRow struct {
	ID               string     `gorm:"type:uuid;primaryKey"`
	DeduplicationKey string     `gorm:"type:text;not null;uniqueIndex"`
	Kind             string     `gorm:"type:text;not null"`
	IdentityProvider string     `gorm:"type:text;not null"`
	ProviderUserID   string     `gorm:"type:text;not null;default:''"`
	Cursor           int        `gorm:"not null;default:0"`
	PageSize         int        `gorm:"not null"`
	ProcessedCount   int        `gorm:"not null;default:0"`
	Status           string     `gorm:"type:text;not null"`
	Attempts         int        `gorm:"not null;default:0"`
	AvailableAt      time.Time  `gorm:"not null"`
	LeaseOwner       string     `gorm:"type:text;not null;default:''"`
	LeaseToken       int64      `gorm:"not null;default:0"`
	LeaseExpiresAt   *time.Time `gorm:"type:timestamptz"`
	LastError        string     `gorm:"type:text;not null;default:''"`
	CreatedAt        time.Time  `gorm:"not null"`
	UpdatedAt        time.Time  `gorm:"not null"`
	CompletedAt      *time.Time `gorm:"type:timestamptz"`
	DeadAt           *time.Time `gorm:"type:timestamptz"`
}

func (userSyncJobRow) TableName() string { return "public.user_sync_jobs" }

type userSyncJobRepository struct{ db *gorm.DB }

func NewUserSyncJobRepository(db *platformpostgres.DB) port.UserSyncJobRepository {
	return &userSyncJobRepository{db: db.DB}
}

func (r *userSyncJobRepository) Enqueue(ctx context.Context, job *domain.UserSyncJob) (bool, error) {
	if err := validateSyncJob(job); err != nil {
		return false, err
	}
	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.Status == "" {
		job.Status = domain.UserSyncJobPending
	}
	if job.AvailableAt.IsZero() {
		job.AvailableAt = now
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now

	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := syncJobToRow(job)
	result := activeDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "deduplication_key"}},
		DoNothing: true,
	}).Create(&row)
	if result.Error != nil {
		return false, fmt.Errorf("user_sync_job_repository: enqueue %q: %w", job.DeduplicationKey, result.Error)
	}
	if result.RowsAffected == 1 {
		*job = *syncJobToDomain(&row)
		return true, nil
	}

	row = userSyncJobRow{}
	if err := activeDB.WithContext(ctx).
		Where("deduplication_key = ?", job.DeduplicationKey).
		First(&row).Error; err != nil {
		return false, fmt.Errorf("user_sync_job_repository: load existing %q: %w", job.DeduplicationKey, err)
	}
	existing := syncJobToDomain(&row)
	if existing.Kind != job.Kind || existing.IdentityProvider != job.IdentityProvider ||
		existing.ProviderUserID != job.ProviderUserID {
		return false, fmt.Errorf("user_sync_job_repository: deduplication key %q has conflicting payload", job.DeduplicationKey)
	}
	*job = *existing
	return false, nil
}

func (r *userSyncJobRepository) ClaimNext(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (*domain.UserSyncJob, error) {
	if strings.TrimSpace(workerID) == "" {
		return nil, errors.New("user_sync_job_repository: worker ID is required")
	}
	if leaseDuration <= 0 {
		return nil, errors.New("user_sync_job_repository: lease duration must be positive")
	}

	now := time.Now().UTC()
	leaseExpiresAt := now.Add(leaseDuration)
	var row userSyncJobRow
	result := r.db.WithContext(ctx).Raw(`
		WITH picked AS (
			SELECT jobs.id
			FROM public.user_sync_jobs AS jobs
			WHERE (jobs.status = 'PENDING' AND jobs.available_at <= ?)
			   OR (jobs.status = 'PROCESSING' AND jobs.lease_expires_at < ?)
			ORDER BY jobs.available_at, jobs.created_at, jobs.id
			FOR UPDATE OF jobs SKIP LOCKED
			LIMIT 1
		)
		UPDATE public.user_sync_jobs AS jobs
		SET status = 'PROCESSING',
		    lease_owner = ?,
		    lease_token = jobs.lease_token + 1,
		    lease_expires_at = ?,
		    updated_at = ?
		FROM picked
		WHERE jobs.id = picked.id
		RETURNING jobs.*`, now, now, workerID, leaseExpiresAt, now).Scan(&row)
	if result.Error != nil {
		return nil, fmt.Errorf("user_sync_job_repository: claim next: %w", result.Error)
	}
	if result.RowsAffected == 0 || row.ID == "" {
		return nil, nil
	}
	return syncJobToDomain(&row), nil
}

func (r *userSyncJobRepository) Advance(
	ctx context.Context,
	jobID, workerID string,
	leaseToken int64,
	nextCursor, processedDelta int,
	completed bool,
) error {
	if nextCursor < 0 || processedDelta < 0 {
		return errors.New("user_sync_job_repository: cursor and processed delta cannot be negative")
	}
	now := time.Now().UTC()
	status := domain.UserSyncJobPending
	var completedAt any
	if completed {
		status = domain.UserSyncJobCompleted
		completedAt = now
	}
	activeDB := platformpostgres.FromContext(ctx, r.db)
	result := activeDB.WithContext(ctx).Model(&userSyncJobRow{}).
		Where(
			"id = ? AND status = ? AND lease_owner = ? AND lease_token = ? AND cursor <= ?",
			jobID, domain.UserSyncJobProcessing, workerID, leaseToken, nextCursor,
		).
		Updates(map[string]any{
			"cursor": nextCursor, "processed_count": gorm.Expr("processed_count + ?", processedDelta),
			"status": status, "attempts": 0, "available_at": now,
			"lease_owner": "", "lease_expires_at": nil, "last_error": "",
			"updated_at": now, "completed_at": completedAt, "dead_at": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("user_sync_job_repository: advance %s: %w", jobID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("user_sync_job_repository: advance %s: %w", jobID, port.ErrUserSyncJobLeaseLost)
	}
	return nil
}

func (r *userSyncJobRepository) MarkRetry(
	ctx context.Context,
	jobID, workerID string,
	leaseToken int64,
	failure string,
	delay time.Duration,
	maxAttempts int,
) error {
	if maxAttempts <= 0 {
		maxAttempts = 12
	}
	if delay < 0 {
		delay = 0
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Exec(`
		UPDATE public.user_sync_jobs
		SET attempts = attempts + 1,
		    status = CASE WHEN attempts + 1 >= ? THEN 'DEAD' ELSE 'PENDING' END,
		    available_at = CASE WHEN attempts + 1 >= ? THEN available_at ELSE ? END,
		    lease_owner = '',
		    lease_expires_at = NULL,
		    last_error = ?,
		    dead_at = CASE WHEN attempts + 1 >= ? THEN ? ELSE NULL END,
		    updated_at = ?
		WHERE id = ? AND status = 'PROCESSING' AND lease_owner = ? AND lease_token = ?`,
		maxAttempts, maxAttempts, now.Add(delay), truncateSyncJobError(failure, 4096), maxAttempts, now, now,
		jobID, workerID, leaseToken,
	)
	if result.Error != nil {
		return fmt.Errorf("user_sync_job_repository: retry %s: %w", jobID, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("user_sync_job_repository: retry %s: %w", jobID, port.ErrUserSyncJobLeaseLost)
	}
	return nil
}

func validateSyncJob(job *domain.UserSyncJob) error {
	if job == nil {
		return errors.New("user_sync_job_repository: job is required")
	}
	if strings.TrimSpace(job.DeduplicationKey) == "" || strings.TrimSpace(job.IdentityProvider) == "" {
		return errors.New("user_sync_job_repository: deduplication key and identity provider are required")
	}
	if job.PageSize <= 0 || job.PageSize > 1000 {
		return errors.New("user_sync_job_repository: page size must be between 1 and 1000")
	}
	switch job.Kind {
	case domain.UserSyncJobSingle:
		if strings.TrimSpace(job.ProviderUserID) == "" {
			return errors.New("user_sync_job_repository: provider user ID is required for a single-user job")
		}
	case domain.UserSyncJobFull:
		if job.ProviderUserID != "" {
			return errors.New("user_sync_job_repository: provider user ID must be empty for a full job")
		}
	default:
		return fmt.Errorf("user_sync_job_repository: unsupported job kind %q", job.Kind)
	}
	return nil
}

func syncJobToRow(job *domain.UserSyncJob) userSyncJobRow {
	return userSyncJobRow{
		ID: job.ID, DeduplicationKey: job.DeduplicationKey, Kind: string(job.Kind),
		IdentityProvider: job.IdentityProvider, ProviderUserID: job.ProviderUserID,
		Cursor: job.Cursor, PageSize: job.PageSize, ProcessedCount: job.ProcessedCount,
		Status: string(job.Status), Attempts: job.Attempts, AvailableAt: job.AvailableAt,
		LeaseOwner: job.LeaseOwner, LeaseToken: job.LeaseToken, LeaseExpiresAt: job.LeaseExpiresAt,
		LastError: job.LastError, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
		CompletedAt: job.CompletedAt, DeadAt: job.DeadAt,
	}
}

func syncJobToDomain(row *userSyncJobRow) *domain.UserSyncJob {
	return &domain.UserSyncJob{
		ID: row.ID, DeduplicationKey: row.DeduplicationKey, Kind: domain.UserSyncJobKind(row.Kind),
		IdentityProvider: row.IdentityProvider, ProviderUserID: row.ProviderUserID,
		Cursor: row.Cursor, PageSize: row.PageSize, ProcessedCount: row.ProcessedCount,
		Status: domain.UserSyncJobStatus(row.Status), Attempts: row.Attempts, AvailableAt: row.AvailableAt,
		LeaseOwner: row.LeaseOwner, LeaseToken: row.LeaseToken, LeaseExpiresAt: row.LeaseExpiresAt,
		LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		CompletedAt: row.CompletedAt, DeadAt: row.DeadAt,
	}
}

func truncateSyncJobError(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
