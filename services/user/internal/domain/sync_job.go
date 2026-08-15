package domain

import "time"

// UserSyncJobKind identifies the amount of identity-provider data fetched by a
// durable synchronization job.
type UserSyncJobKind string

const (
	UserSyncJobSingle UserSyncJobKind = "SINGLE"
	UserSyncJobFull   UserSyncJobKind = "FULL"
)

// UserSyncJobStatus is the durable lifecycle of a synchronization job.
type UserSyncJobStatus string

const (
	UserSyncJobPending    UserSyncJobStatus = "PENDING"
	UserSyncJobProcessing UserSyncJobStatus = "PROCESSING"
	UserSyncJobCompleted  UserSyncJobStatus = "COMPLETED"
	UserSyncJobDead       UserSyncJobStatus = "DEAD"
)

// UserSyncJob is a restart-safe intent to copy one user or a paginated set of
// users from an external identity provider into the User service database.
// LeaseToken is a fencing token: only the worker holding the latest value may
// advance or retry the job.
type UserSyncJob struct {
	ID               string
	DeduplicationKey string
	Kind             UserSyncJobKind
	IdentityProvider string
	ProviderUserID   string
	Cursor           int
	PageSize         int
	ProcessedCount   int
	Status           UserSyncJobStatus
	Attempts         int
	AvailableAt      time.Time
	LeaseOwner       string
	LeaseToken       int64
	LeaseExpiresAt   *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	DeadAt           *time.Time
}
