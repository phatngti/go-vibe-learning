package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"github.com/google/uuid"
)

const keycloakProvider = "keycloak"

// UserSyncJobPolicy contains bounded worker controls. ProviderTimeout must be
// shorter than LeaseDuration so a healthy worker can persist its result before
// another worker is allowed to reclaim the job.
type UserSyncJobPolicy struct {
	PageSize        int
	LeaseDuration   time.Duration
	ProviderTimeout time.Duration
	MaxAttempts     int
	RetryBaseDelay  time.Duration
	RetryMaxDelay   time.Duration
}

func (p UserSyncJobPolicy) Validate() error {
	if p.PageSize <= 0 || p.PageSize > 1000 {
		return errors.New("user sync job page size must be between 1 and 1000")
	}
	if p.ProviderTimeout <= 0 || p.LeaseDuration <= p.ProviderTimeout {
		return errors.New("user sync job lease duration must exceed the positive provider timeout")
	}
	if p.MaxAttempts <= 0 {
		return errors.New("user sync job max attempts must be positive")
	}
	if p.RetryBaseDelay <= 0 || p.RetryMaxDelay < p.RetryBaseDelay {
		return errors.New("user sync job retry delays are invalid")
	}
	return nil
}

// RequestUserSyncHandler persists sync intent without contacting the identity
// provider. When invoked from the event-store consumer with a transaction-bound
// context, job creation and checkpoint advancement commit atomically.
type RequestUserSyncHandler struct {
	jobs   port.UserSyncJobRepository
	policy UserSyncJobPolicy
}

func NewRequestUserSyncHandler(
	jobs port.UserSyncJobRepository,
	policy UserSyncJobPolicy,
) (*RequestUserSyncHandler, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &RequestUserSyncHandler{jobs: jobs, policy: policy}, nil
}

func (h *RequestUserSyncHandler) EnqueueRegistered(
	ctx context.Context,
	eventID, providerUserID string,
) (*domain.UserSyncJob, bool, error) {
	eventID = strings.TrimSpace(eventID)
	providerUserID = strings.TrimSpace(providerUserID)
	if eventID == "" || providerUserID == "" {
		return nil, false, errors.New("request user sync: event ID and provider user ID are required")
	}
	job := &domain.UserSyncJob{
		DeduplicationKey: "user-registered:" + eventID,
		Kind:             domain.UserSyncJobSingle, IdentityProvider: keycloakProvider,
		ProviderUserID: providerUserID, PageSize: 1,
	}
	created, err := h.jobs.Enqueue(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("request user sync for registration %s: %w", eventID, err)
	}
	return job, created, nil
}

// EnqueueFull creates a restart-safe paginated synchronization request. A
// caller-supplied idempotency key makes retries return the existing job.
func (h *RequestUserSyncHandler) EnqueueFull(
	ctx context.Context,
	idempotencyKey string,
) (*domain.UserSyncJob, bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}
	job := &domain.UserSyncJob{
		DeduplicationKey: "full-sync:" + idempotencyKey,
		Kind:             domain.UserSyncJobFull, IdentityProvider: keycloakProvider,
		PageSize: h.policy.PageSize,
	}
	created, err := h.jobs.Enqueue(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("request full user sync: %w", err)
	}
	return job, created, nil
}

// UserSyncJobRunner claims and processes one durable job step. Claiming,
// provider I/O, and persistence are deliberately separate: the external call
// never runs inside a database transaction, while each page and its cursor
// transition commit atomically.
type UserSyncJobRunner struct {
	jobs      port.UserSyncJobRepository
	syncGw    port.SyncGateway
	syncUsers *SyncUsersHandler
	policy    UserSyncJobPolicy
}

func NewUserSyncJobRunner(
	jobs port.UserSyncJobRepository,
	syncGw port.SyncGateway,
	syncUsers *SyncUsersHandler,
	policy UserSyncJobPolicy,
) (*UserSyncJobRunner, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &UserSyncJobRunner{jobs: jobs, syncGw: syncGw, syncUsers: syncUsers, policy: policy}, nil
}

// RunNext processes at most one single-user job or one page of a full job.
// claimed is true even when processing failed and the job was rescheduled.
func (h *UserSyncJobRunner) RunNext(ctx context.Context, workerID string) (claimed bool, err error) {
	job, err := h.jobs.ClaimNext(ctx, workerID, h.policy.LeaseDuration)
	if err != nil {
		return false, fmt.Errorf("claim user sync job: %w", err)
	}
	if job == nil {
		return false, nil
	}

	if err := h.process(ctx, workerID, job); err != nil {
		delay := syncJobRetryDelay(job.Attempts, h.policy.RetryBaseDelay, h.policy.RetryMaxDelay)
		retryErr := h.jobs.MarkRetry(
			ctx, job.ID, workerID, job.LeaseToken, err.Error(), delay, h.policy.MaxAttempts,
		)
		if retryErr != nil {
			return true, errors.Join(err, fmt.Errorf("reschedule user sync job %s: %w", job.ID, retryErr))
		}
		return true, err
	}
	return true, nil
}

func (h *UserSyncJobRunner) process(ctx context.Context, workerID string, job *domain.UserSyncJob) error {
	providerCtx, cancel := context.WithTimeout(ctx, h.policy.ProviderTimeout)
	defer cancel()

	switch job.Kind {
	case domain.UserSyncJobSingle:
		user, err := h.syncGw.FetchUserByID(providerCtx, job.ProviderUserID)
		if err != nil {
			return fmt.Errorf("fetch provider user %s: %w", job.ProviderUserID, err)
		}
		if user == nil {
			return fmt.Errorf("fetch provider user %s: empty response", job.ProviderUserID)
		}
		if user.IdentityProviderUserID != job.ProviderUserID {
			return fmt.Errorf(
				"fetch provider user %s: response identity mismatch %q",
				job.ProviderUserID, user.IdentityProviderUserID,
			)
		}
		return h.syncUsers.SyncBatch(ctx, []domain.UserIdentity{*user}, func(txCtx context.Context) error {
			return h.jobs.Advance(txCtx, job.ID, workerID, job.LeaseToken, job.Cursor+1, 1, true)
		})

	case domain.UserSyncJobFull:
		page, err := h.syncGw.FetchUsersPage(providerCtx, job.Cursor, job.PageSize)
		if err != nil {
			return fmt.Errorf("fetch provider users at cursor %d: %w", job.Cursor, err)
		}
		if page.NextCursor != job.Cursor+len(page.Users) {
			return fmt.Errorf(
				"fetch provider users at cursor %d: invalid next cursor %d for %d users",
				job.Cursor, page.NextCursor, len(page.Users),
			)
		}
		if !page.Done && len(page.Users) == 0 {
			return fmt.Errorf("fetch provider users at cursor %d: empty non-terminal page", job.Cursor)
		}
		return h.syncUsers.SyncBatch(ctx, page.Users, func(txCtx context.Context) error {
			return h.jobs.Advance(
				txCtx, job.ID, workerID, job.LeaseToken,
				page.NextCursor, len(page.Users), page.Done,
			)
		})

	default:
		return fmt.Errorf("unsupported user sync job kind %q", job.Kind)
	}
}

func syncJobRetryDelay(attempts int, base, maximum time.Duration) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	delay := base
	for i := 0; i < attempts && delay < maximum; i++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
