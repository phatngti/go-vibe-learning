package postgres

import "time"

type AgentConfigRow struct {
	AgentID                    string     `gorm:"type:text;primaryKey"`
	TenantID                   string     `gorm:"type:text"`
	CallbackURL                string     `gorm:"type:text;not null"`
	CallbackHost               string     `gorm:"type:text;not null;default:''"`
	CallbackStatus             string     `gorm:"type:text;not null;default:'PENDING_VERIFICATION'"`
	CallbackVerifiedAt         *time.Time `gorm:"type:timestamptz"`
	CallbackChallengeID        string     `gorm:"type:text;not null;default:''"`
	CallbackChallengeDigest    string     `gorm:"type:text;not null;default:''"`
	CallbackChallengeIssuedAt  *time.Time `gorm:"type:timestamptz"`
	CallbackChallengeExpiresAt *time.Time `gorm:"type:timestamptz"`
	CallbackChallengeUsedAt    *time.Time `gorm:"type:timestamptz"`
	ApiKey                     string     `gorm:"type:text;uniqueIndex;not null"`
	AllowedIPs                 string     `gorm:"type:text;default:''"`
	UpdatedAt                  time.Time  `gorm:"not null;default:now()"`
}

func (AgentConfigRow) TableName() string { return "public.agent_configs" }

type AuditLogRow struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AgentID   string    `gorm:"type:text;not null"`
	TenantID  string    `gorm:"type:text"`
	Action    string    `gorm:"type:text;not null"`
	Details   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (AuditLogRow) TableName() string { return "public.agent_audit_logs" }

type CallbackDeliveryRow struct {
	TransactionID      string     `gorm:"type:text;primaryKey"`
	AgentID            string     `gorm:"type:text;not null"`
	TenantID           string     `gorm:"type:text"`
	EventID            string     `gorm:"type:text;not null;default:'';uniqueIndex"`
	CallbackURL        string     `gorm:"type:text"`
	Payload            []byte     `gorm:"type:bytea;not null"`
	KeyVersion         string     `gorm:"type:text;not null;default:''"`
	Status             string     `gorm:"type:text;not null"`
	Attempts           int        `gorm:"not null;default:0"`
	GenerationAttempts int        `gorm:"not null;default:0"`
	NextAttemptAt      *time.Time `gorm:"type:timestamptz"`
	LastAttemptAt      *time.Time `gorm:"type:timestamptz"`
	LeaseOwner         string     `gorm:"type:text;not null;default:''"`
	LeaseToken         int64      `gorm:"not null;default:0"`
	LeaseExpiresAt     *time.Time `gorm:"type:timestamptz"`
	ReplayCount        int        `gorm:"not null;default:0"`
	OutcomeGeneration  int64      `gorm:"not null;default:1"`
	LastError          string     `gorm:"type:text"`
	CreatedAt          time.Time  `gorm:"not null;default:now()"`
	UpdatedAt          time.Time  `gorm:"not null;default:now()"`
	TerminalAt         *time.Time `gorm:"type:timestamptz"`
}

func (CallbackDeliveryRow) TableName() string { return "public.callback_deliveries" }

type CallbackDeadLetterRow struct {
	ID                uint       `gorm:"primaryKey;autoIncrement"`
	EventID           string     `gorm:"type:text;not null;uniqueIndex:uq_callback_dead_letters_generation"`
	TransactionID     string     `gorm:"type:text;not null"`
	AgentID           string     `gorm:"type:text;not null"`
	TenantID          string     `gorm:"type:text;not null"`
	OutcomeGeneration int64      `gorm:"not null;uniqueIndex:uq_callback_dead_letters_generation"`
	Attempts          int        `gorm:"not null"`
	Reason            string     `gorm:"type:text;not null"`
	Status            string     `gorm:"type:text;not null"`
	CreatedAt         time.Time  `gorm:"not null"`
	ReplayedAt        *time.Time `gorm:"type:timestamptz"`
	ReplayedBy        string     `gorm:"type:text;not null;default:''"`
	ReplayReason      string     `gorm:"type:text;not null;default:''"`
}

func (CallbackDeadLetterRow) TableName() string { return "public.callback_dead_letters" }
