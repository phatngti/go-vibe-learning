package domain

import "time"

// AgentConfig holds the configuration for an API agent.
type AgentConfig struct {
	AgentID            string
	TenantID           string
	CallbackURL        string
	CallbackHost       string
	CallbackStatus     string
	CallbackVerifiedAt *time.Time
	// CallbackChallengeDigest is the SHA-256 digest of a high-entropy token.
	// The plaintext token is returned once when the endpoint is configured and
	// is never persisted.
	CallbackChallengeID        string
	CallbackChallengeDigest    string
	CallbackChallengeIssuedAt  *time.Time
	CallbackChallengeExpiresAt *time.Time
	CallbackChallengeUsedAt    *time.Time
	ApiKey                     string
	AllowedIPs                 string
	UpdatedAt                  time.Time
}

// AuditLog records agent actions for compliance.
type AuditLog struct {
	ID        uint
	AgentID   string
	TenantID  string
	Action    string
	Details   string
	CreatedAt time.Time
}

// CallbackDelivery tracks webhook delivery status.
type CallbackDelivery struct {
	TransactionID string
	AgentID       string
	TenantID      string
	EventID       string
	CallbackURL   string
	Payload       []byte
	KeyVersion    string
	Status        string
	Attempts      int
	// GenerationAttempts is reset by an audited DLQ replay. Attempts remains a
	// lifetime counter for operations and audit reporting.
	GenerationAttempts int
	NextAttemptAt      *time.Time
	LastAttemptAt      *time.Time
	LeaseOwner         string
	LeaseToken         int64
	LeaseExpiresAt     *time.Time
	ReplayCount        int
	OutcomeGeneration  int64
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	TerminalAt         *time.Time
}

// CallbackDeadLetter is an immutable record of one exhausted delivery
// generation. Replaying a dead letter updates its replay audit fields but does
// not change the original callback event identity or body.
type CallbackDeadLetter struct {
	ID                uint
	EventID           string
	TransactionID     string
	AgentID           string
	TenantID          string
	OutcomeGeneration int64
	Attempts          int
	Reason            string
	Status            string
	CreatedAt         time.Time
	ReplayedAt        *time.Time
	ReplayedBy        string
	ReplayReason      string
}

const (
	CallbackStatusPending  = "PENDING_VERIFICATION"
	CallbackStatusVerified = "VERIFIED"
)

const (
	CallbackDeliveryPending        = "PENDING"
	CallbackDeliveryInFlight       = "IN_FLIGHT"
	CallbackDeliveryRetryScheduled = "RETRY_SCHEDULED"
	CallbackDeliveryDelivered      = "DELIVERED"
	CallbackDeliverySkipped        = "SKIPPED"
	CallbackDeliveryTerminalFailed = "TERMINAL_FAILED"

	CallbackDeadLetterOpen     = "OPEN"
	CallbackDeadLetterReplayed = "REPLAYED"
)

type CallbackChallenge struct {
	AgentID     string
	TenantID    string
	ChallengeID string
	Token       string
	ProofURL    string
	ExpiresAt   time.Time
	Status      string
}

type CallbackChallengeInstructions struct {
	AgentID     string
	TenantID    string
	ChallengeID string
	ProofURL    string
	ExpiresAt   time.Time
	Status      string
}

type CallbackChallengeSecret struct {
	ID     string
	Token  string
	Digest string
}

type CallbackConfiguration struct {
	Config    *AgentConfig
	Challenge CallbackChallenge
}

type ValidatedCallbackEndpoint struct {
	URL      string
	Hostname string
	Verified bool
}

type CallbackSigningInput struct {
	EventID       string
	TenantID      string
	TransactionID string
	Body          []byte
}

type CallbackSignature struct {
	Timestamp  string
	BodySHA256 string
	KeyVersion string
	Signature  string
}
