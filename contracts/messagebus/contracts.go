// File: internal/platform/messagebus/contracts.go
// Description: Type and step constant definitions representing messages exchanged across services.

package messagebus

import "fmt"

const (
	PaymentInitiatedType           = "PaymentInitiated"
	PaymentCompletedType           = "PaymentCompleted"
	PaymentFailedType              = "PaymentFailed"
	PaymentProcessingUncertainType = "PaymentProcessingUncertain"
	AccumulateVolumeRequestedType  = "AccumulateVolumeRequested"
	VolumeAccumulatedType          = "VolumeAccumulated"
	CreditCommissionRequestedType  = "CreditCommissionRequested"
	CommissionCreditedType         = "CommissionCredited"
	DispatchCallbackRequestedType  = "DispatchCallbackRequested"
	CallbackDispatchedType         = "CallbackDispatched"
	CallbackDispatchSkippedType    = "CallbackDispatchSkipped"
	CallbackDispatchFailedType     = "CallbackDispatchFailed"
	SagaStepFailedType             = "SagaStepFailed"
	DeadLetterType                 = "DeadLetter"

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
	TransactionID string `json:"transaction_id"`
	AgentID       string `json:"agent_id"`
	CustomerID    string `json:"customer_id,omitempty"`
	Money         *Money `json:"money,omitempty"`
	// Amount and Currency are retained during the v1 compatibility window.
	// New consumers must prefer Money and only fall back to these fields when
	// decoding a legacy producer.
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	CallbackURL string  `json:"callback_url,omitempty"`
}

type PaymentCompleted struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	CustomerID    string  `json:"customer_id,omitempty"`
	Money         *Money  `json:"money,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

type PaymentFailed struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Money         *Money  `json:"money,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Reason        string  `json:"reason"`
}

type PaymentProcessingUncertain struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Money         *Money  `json:"money,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Reason        string  `json:"reason"`
}

type AccumulateVolumeRequested struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Money         *Money  `json:"money,omitempty"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

type VolumeAccumulated struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Money         *Money  `json:"money,omitempty"`
	TotalMoney    *Money  `json:"total_money,omitempty"`
	Amount        float64 `json:"amount"`
	TotalVolume   float64 `json:"total_volume"`
	CurrentRank   string  `json:"current_rank"`
	RankPromoted  bool    `json:"rank_promoted"`
}

type CreditCommissionRequested struct {
	TransactionID     string  `json:"transaction_id"`
	AgentID           string  `json:"agent_id"`
	TransactionMoney  *Money  `json:"transaction_money,omitempty"`
	TransactionAmount float64 `json:"transaction_amount"`
	Currency          string  `json:"currency,omitempty"`
	Rank              string  `json:"rank"`
}

type CommissionCredited struct {
	TransactionID    string  `json:"transaction_id"`
	AgentID          string  `json:"agent_id"`
	CommissionMoney  *Money  `json:"commission_money,omitempty"`
	BalanceMoney     *Money  `json:"balance_money,omitempty"`
	CommissionAmount float64 `json:"commission_amount"`
	NewBalance       float64 `json:"new_balance"`
}

type DispatchCallbackRequested struct {
	TransactionID string  `json:"transaction_id"`
	AgentID       string  `json:"agent_id"`
	Money         *Money  `json:"money,omitempty"`
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
	SourceTopic         string `json:"source_topic"`
	SourceConsumer      string `json:"source_consumer"`
	SourcePartition     int    `json:"source_partition"`
	SourceOffset        int64  `json:"source_offset"`
	SourceMessageID     string `json:"source_message_id,omitempty"`
	SourceKey           []byte `json:"source_key,omitempty"`
	ReasonClass         string `json:"reason_class"`
	Reason              string `json:"reason"`
	RawMessage          []byte `json:"raw_message,omitempty"`
	RawMessageSHA256    string `json:"raw_message_sha256"`
	RawMessageTruncated bool   `json:"raw_message_truncated,omitempty"`
}
