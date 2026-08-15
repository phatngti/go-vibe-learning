package port

import (
	"context"
	"time"

	"github/master_go_cqrs/services/agent/internal/domain"
)

// AgentRepository provides access to tenant-owned agent configuration and audit logs.
type AgentRepository interface {
	GetConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error)
	GetConfigByApiKey(ctx context.Context, apiKey string) (*domain.AgentConfig, error)
	SaveConfig(ctx context.Context, cfg *domain.AgentConfig) error
	CreateAuditLog(ctx context.Context, log *domain.AuditLog) error
}

// UnitOfWork provides transactional boundaries.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}

// CallbackEndpointValidator normalizes callback URLs and rejects destinations
// that are unsafe for server-side requests.
type CallbackEndpointValidator interface {
	Validate(ctx context.Context, rawURL string) (domain.ValidatedCallbackEndpoint, error)
}

// CallbackSigner signs the exact body delivered to a tenant endpoint. Secrets
// are owned by the adapter and never cross into application DTOs or persistence.
type CallbackSigner interface {
	Sign(input domain.CallbackSigningInput) (domain.CallbackSignature, error)
}

const (
	CallbackHeaderEventID       = "X-CQRS-Event-ID"
	CallbackHeaderTimestamp     = "X-CQRS-Timestamp"
	CallbackHeaderKeyVersion    = "X-CQRS-Key-Version"
	CallbackHeaderContentSHA256 = "X-CQRS-Content-SHA256"
	CallbackHeaderSignature     = "X-CQRS-Signature"
)

type CallbackDispatchRequest struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

// CallbackDispatcher owns the network side effect. Transport handlers depend
// on this application port rather than an adapter-specific HTTP client.
type CallbackDispatcher interface {
	Dispatch(ctx context.Context, request CallbackDispatchRequest) (statusCode int, err error)
}

// CallbackChallengeGenerator issues a one-time secret and its digest. Only the
// digest crosses the persistence boundary.
type CallbackChallengeGenerator interface {
	Generate() (domain.CallbackChallengeSecret, error)
	Digest(token string) string
}

// CallbackOwnershipProver retrieves a pre-published proof from the configured
// callback origin. Implementations must apply the same SSRF and DNS-rebinding
// protections as callback delivery.
type CallbackOwnershipProver interface {
	Verify(ctx context.Context, proofURL, expectedToken string) error
}

type ScheduleCallbackParams struct {
	ConsumerName string
	MessageID    string
	Topic        string
	MessageType  string
	Delivery     *domain.CallbackDelivery
	Result       *CallbackResultIntent
}

type CallbackResultIntent struct {
	MessageID     string
	Topic         string
	MessageKey    string
	MessageType   string
	AggregateType string
	AggregateID   string
	TenantID      string
	CausationID   string
	Payload       []byte
}

type CallbackReplayRequest struct {
	EventID     string
	TenantID    string
	OperatorID  string
	Reason      string
	AvailableAt time.Time
}

// CallbackDeliveryRepository is the durable scheduling boundary. Every
// transition validates a lease token; terminal transitions atomically persist
// the DLQ row and outgoing result intent.
type CallbackDeliveryRepository interface {
	Schedule(ctx context.Context, params ScheduleCallbackParams) (*domain.CallbackDelivery, bool, error)
	GetByEventID(ctx context.Context, eventID string) (*domain.CallbackDelivery, error)
	ClaimNext(ctx context.Context, workerID string, now time.Time, leaseDuration time.Duration) (*domain.CallbackDelivery, error)
	MarkDelivered(ctx context.Context, delivery *domain.CallbackDelivery, result CallbackResultIntent, now time.Time) error
	MarkRetry(ctx context.Context, delivery *domain.CallbackDelivery, reason string, nextAttemptAt time.Time, now time.Time) error
	MarkTerminal(ctx context.Context, delivery *domain.CallbackDelivery, reason string, result CallbackResultIntent, now time.Time) error
	ReplayTerminal(ctx context.Context, request CallbackReplayRequest) (*domain.CallbackDelivery, error)
}
