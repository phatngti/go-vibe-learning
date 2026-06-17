package payment

type State string

const (
	StateWaitingPayment    State = "WAITING_PAYMENT"
	StateWaitingAffiliate  State = "WAITING_AFFILIATE"
	StateWaitingSettlement State = "WAITING_SETTLEMENT"
	StateWaitingCallback   State = "WAITING_CALLBACK"
	StateCompleted         State = "COMPLETED"
	StateCallbackFailed    State = "COMPLETED_WITH_CALLBACK_FAILURE"
	StateFailed            State = "FAILED"
)

type Snapshot struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Rank          string  `json:"rank,omitempty"`
	State         State   `json:"state"`
	LastError     string  `json:"last_error,omitempty"`
}

func (s Snapshot) Terminal() bool {
	return s.State == StateCompleted || s.State == StateCallbackFailed || s.State == StateFailed
}

type EventKind string

const (
	EventPaymentInitiated   EventKind = "PAYMENT_INITIATED"
	EventPaymentCompleted   EventKind = "PAYMENT_COMPLETED"
	EventVolumeAccumulated  EventKind = "VOLUME_ACCUMULATED"
	EventCommissionCredited EventKind = "COMMISSION_CREDITED"
	EventCallbackCompleted  EventKind = "CALLBACK_COMPLETED"
)

type Event struct {
	Kind           EventKind `json:"kind"`
	TransactionID  string    `json:"transaction_id"`
	AgentID        string    `json:"agent_id"`
	Amount         float64   `json:"amount,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	Rank           string    `json:"rank,omitempty"`
	CallbackStatus string    `json:"callback_status,omitempty"`
	CallbackReason string    `json:"callback_reason,omitempty"`
	CallbackFailed bool      `json:"callback_failed,omitempty"`
}

type CommandKind string

const (
	CommandAccumulateVolume CommandKind = "ACCUMULATE_VOLUME"
	CommandCreditCommission CommandKind = "CREDIT_COMMISSION"
	CommandDispatchCallback CommandKind = "DISPATCH_CALLBACK"
)

type Command struct {
	Kind          CommandKind `json:"kind"`
	TransactionID string      `json:"transaction_id"`
	AgentID       string      `json:"agent_id"`
	Amount        float64     `json:"amount"`
	Currency      string      `json:"currency,omitempty"`
	Rank          string      `json:"rank,omitempty"`
}
