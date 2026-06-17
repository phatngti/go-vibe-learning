// File: internal/platform/messagebus/contracts.go
// Description: Type and step constant definitions representing messages exchanged across services.

package messagebus

import "fmt"

const (
	PaymentInitiatedType          = "PaymentInitiated"
	PaymentCompletedType          = "PaymentCompleted"
	AccumulateVolumeRequestedType = "AccumulateVolumeRequested"
	VolumeAccumulatedType         = "VolumeAccumulated"
	CreditCommissionRequestedType = "CreditCommissionRequested"
	CommissionCreditedType        = "CommissionCredited"
	DispatchCallbackRequestedType = "DispatchCallbackRequested"
	CallbackDispatchedType        = "CallbackDispatched"
	CallbackDispatchSkippedType   = "CallbackDispatchSkipped"
	CallbackDispatchFailedType    = "CallbackDispatchFailed"
	SagaStepFailedType            = "SagaStepFailed"
	DeadLetterType                = "DeadLetter"

	// User service event types
	UserRegisteredType = "UserRegistered"
	UserSyncedType     = "UserSynced"
	UserDisabledType   = "UserDisabled"
)

const (
	StepAffiliate  = "affiliate"
	StepSettlement = "settlement"
	StepCallback   = "callback"
	StepUserSync   = "user-sync"
)

func MessageID(transactionID, step string) string {
	return fmt.Sprintf("%s:%s", transactionID, step)
}

type PaymentInitiated struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	CustomerID    string  `json:"customer_id,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	CallbackURL   string  `json:"callback_url,omitempty"`
}

type PaymentCompleted struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	CustomerID    string  `json:"customer_id,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

type AccumulateVolumeRequested struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

type VolumeAccumulated struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Amount        float64 `json:"amount"`
	TotalVolume   float64 `json:"total_volume"`
	CurrentRank   string  `json:"current_rank"`
	RankPromoted  bool    `json:"rank_promoted"`
}

type CreditCommissionRequested struct {
	TransactionID     string  `json:"transaction_id"`
	AgentID           string  `json:"agent_id"`
	TransactionAmount float64 `json:"transaction_amount"`
	Rank              string  `json:"rank"`
}

type CommissionCredited struct {
	TransactionID    string  `json:"transaction_id"`
	AgentID          string  `json:"agent_id"`
	CommissionAmount float64 `json:"commission_amount"`
	NewBalance       float64 `json:"new_balance"`
}

type DispatchCallbackRequested struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

type CallbackResult struct {
	TransactionID string `json:"transaction_id"`
	AgentID       string `json:"agent_id"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
}

type SagaStepFailed struct {
	TransactionID string `json:"transaction_id"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
}

type DeadLetter struct {
	SourceTopic string `json:"source_topic"`
	Reason      string `json:"reason"`
	RawMessage  []byte `json:"raw_message,omitempty"`
}
