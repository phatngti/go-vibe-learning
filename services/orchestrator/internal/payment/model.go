package payment

import (
	"fmt"
	"strconv"
	"strings"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

type State string

const (
	StateWaitingPayment    State = "WAITING_PAYMENT"
	StateWaitingAffiliate  State = "WAITING_AFFILIATE"
	StateWaitingSettlement State = "WAITING_SETTLEMENT"
	StateWaitingCallback   State = "WAITING_CALLBACK"
	StateNeedsManualReview State = "NEEDS_MANUAL_REVIEW"
	StateCompleted         State = "COMPLETED"
	StateCallbackFailed    State = "COMPLETED_WITH_CALLBACK_FAILURE"
	StateFailed            State = "FAILED"
)

type Snapshot struct {
	TransactionID           string             `json:"transaction_id"`
	AgentID                 string             `json:"agent_id"`
	Money                   *sharedmoney.Money `json:"money,omitempty"`
	Amount                  float64            `json:"amount"`
	Currency                string             `json:"currency"`
	Rank                    string             `json:"rank,omitempty"`
	State                   State              `json:"state"`
	OrchestrationOwner      string             `json:"orchestration_owner,omitempty"`
	OrchestrationGeneration int64              `json:"orchestration_generation,omitempty"`
	// ResumeState preserves the step that was awaiting a definitive outcome
	// when the retry budget was exhausted. A late authoritative event can then
	// safely resume the process instead of being discarded as terminal.
	ResumeState State  `json:"resume_state,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

func (s Snapshot) Terminal() bool {
	return s.State == StateCompleted || s.State == StateCallbackFailed || s.State == StateFailed
}

type EventKind string

const (
	EventPaymentInitiated   EventKind = "PAYMENT_INITIATED"
	EventPaymentCompleted   EventKind = "PAYMENT_COMPLETED"
	EventPaymentFailed      EventKind = "PAYMENT_FAILED"
	EventPaymentUncertain   EventKind = "PAYMENT_PROCESSING_UNCERTAIN"
	EventVolumeAccumulated  EventKind = "VOLUME_ACCUMULATED"
	EventCommissionCredited EventKind = "COMMISSION_CREDITED"
	EventCallbackCompleted  EventKind = "CALLBACK_COMPLETED"
)

type Event struct {
	MessageID               string             `json:"message_id,omitempty"`
	Kind                    EventKind          `json:"kind"`
	TransactionID           string             `json:"transaction_id"`
	AgentID                 string             `json:"agent_id"`
	Money                   *sharedmoney.Money `json:"money,omitempty"`
	Amount                  float64            `json:"amount,omitempty"`
	Currency                string             `json:"currency,omitempty"`
	Rank                    string             `json:"rank,omitempty"`
	CallbackStatus          string             `json:"callback_status,omitempty"`
	CallbackReason          string             `json:"callback_reason,omitempty"`
	CallbackFailed          bool               `json:"callback_failed,omitempty"`
	Reason                  string             `json:"reason,omitempty"`
	OrchestrationOwner      string             `json:"orchestration_owner,omitempty"`
	OrchestrationGeneration int64              `json:"orchestration_generation,omitempty"`
}

type CommandKind string

const (
	CommandAccumulateVolume CommandKind = "ACCUMULATE_VOLUME"
	CommandCreditCommission CommandKind = "CREDIT_COMMISSION"
	CommandDispatchCallback CommandKind = "DISPATCH_CALLBACK"
)

type Command struct {
	Kind                    CommandKind        `json:"kind"`
	TransactionID           string             `json:"transaction_id"`
	AgentID                 string             `json:"agent_id"`
	Money                   *sharedmoney.Money `json:"money,omitempty"`
	Amount                  float64            `json:"amount"`
	Currency                string             `json:"currency,omitempty"`
	Rank                    string             `json:"rank,omitempty"`
	OrchestrationOwner      string             `json:"orchestration_owner,omitempty"`
	OrchestrationGeneration int64              `json:"orchestration_generation,omitempty"`
}

func (s Snapshot) MonetaryAmount() (sharedmoney.Money, error) {
	return resolveMoney(s.Money, s.Amount, s.Currency)
}

func (e Event) MonetaryAmount() (sharedmoney.Money, error) {
	return resolveMoney(e.Money, e.Amount, e.Currency)
}

func (c Command) MonetaryAmount() (sharedmoney.Money, error) {
	return resolveMoney(c.Money, c.Amount, c.Currency)
}

func resolveMoney(preferred *sharedmoney.Money, legacyAmount float64, legacyCurrency string) (sharedmoney.Money, error) {
	if preferred == nil {
		return sharedmoney.FromLegacyFloat64(legacyAmount, legacyCurrency)
	}
	value, err := sharedmoney.New(preferred.AmountMinor, preferred.Currency, preferred.Exponent)
	if err != nil {
		return sharedmoney.Money{}, err
	}
	if legacyCurrency != "" && !strings.EqualFold(strings.TrimSpace(legacyCurrency), value.Currency) {
		return sharedmoney.Money{}, fmt.Errorf("money currency %s conflicts with legacy currency %s", value.Currency, legacyCurrency)
	}
	return value, nil
}

func moneyPointer(value sharedmoney.Money) *sharedmoney.Money {
	copy := value
	return &copy
}

func legacyMajorFloat(value sharedmoney.Money) (float64, error) {
	rendered, err := value.MajorString()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(rendered, 64)
}
