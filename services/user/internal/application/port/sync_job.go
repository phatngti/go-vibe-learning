package port

import (
	"context"
	"errors"
	"time"

	"github/master_go_cqrs/services/user/internal/domain"
)

var ErrUserSyncJobLeaseLost = errors.New("user sync job processing lease lost")

// UserSyncJobRepository stores durable sync intents and owns their
// lease/fencing transitions. Implementations must honor a UnitOfWork-bound
// transaction carried by ctx for Enqueue and Advance.
type UserSyncJobRepository interface {
	// Enqueue inserts a job once per deduplication key. It writes the persisted
	// job (new or existing) back to job and reports whether it was newly created.
	Enqueue(ctx context.Context, job *domain.UserSyncJob) (created bool, err error)

	// ClaimNext atomically leases one available job. Expired PROCESSING jobs are
	// reclaimable and receive a new monotonically increasing fencing token.
	ClaimNext(ctx context.Context, workerID string, leaseDuration time.Duration) (*domain.UserSyncJob, error)

	// Advance commits page progress while the caller still owns the lease. When
	// completed is false, the job becomes immediately available for its next page.
	Advance(
		ctx context.Context,
		jobID, workerID string,
		leaseToken int64,
		nextCursor, processedDelta int,
		completed bool,
	) error

	// MarkRetry releases a claimed job with bounded exponential retry. Once the
	// maximum attempt count is reached the job moves to DEAD.
	MarkRetry(
		ctx context.Context,
		jobID, workerID string,
		leaseToken int64,
		failure string,
		delay time.Duration,
		maxAttempts int,
	) error
}
