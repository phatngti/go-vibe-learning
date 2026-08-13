package codec

import (
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/contracts/messagebus"
	platformkafka "github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/config"
	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
)

type EncodedCommand struct {
	Topic   string
	Key     string
	Message *platformkafka.Message
}

type Kafka struct {
	topics config.KafkaTopics
}

func NewKafka(cfg *config.Config) *Kafka {
	return &Kafka{topics: cfg.Kafka.Topics}
}

func (c *Kafka) Decode(message *platformkafka.Message) (payment.Event, error) {
	var event payment.Event
	switch message.EventType {
	case messagebus.PaymentInitiatedType:
		var payload messagebus.PaymentInitiated
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		event = payment.Event{
			Kind: payment.EventPaymentInitiated, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Amount: payload.Amount, Currency: payload.Currency,
		}
	case messagebus.PaymentCompletedType:
		var payload messagebus.PaymentCompleted
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		event = payment.Event{
			Kind: payment.EventPaymentCompleted, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Amount: payload.Amount, Currency: payload.Currency,
		}
	case messagebus.VolumeAccumulatedType:
		var payload messagebus.VolumeAccumulated
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		event = payment.Event{
			Kind: payment.EventVolumeAccumulated, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Amount: payload.Amount, Rank: payload.CurrentRank,
		}
	case messagebus.CommissionCreditedType:
		var payload messagebus.CommissionCredited
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		event = payment.Event{
			Kind: payment.EventCommissionCredited, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID,
		}
	case messagebus.CallbackDispatchedType, messagebus.CallbackDispatchSkippedType, messagebus.CallbackDispatchFailedType:
		var payload messagebus.CallbackResult
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		event = payment.Event{
			Kind: payment.EventCallbackCompleted, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, CallbackStatus: payload.Status, CallbackReason: payload.Reason,
			CallbackFailed: message.EventType == messagebus.CallbackDispatchFailedType,
		}
	default:
		return event, processcore.Invalid("unsupported Kafka event %q", message.EventType)
	}
	if err := payment.ValidateEvent(event); err != nil {
		return payment.Event{}, err
	}
	return event, nil
}

func (c *Kafka) Encode(command payment.Command) (EncodedCommand, error) {
	var topic, messageType, step string
	var payload any
	switch command.Kind {
	case payment.CommandAccumulateVolume:
		topic, messageType, step = c.topics.AffiliateCommands, messagebus.AccumulateVolumeRequestedType, messagebus.StepAffiliate
		payload = messagebus.AccumulateVolumeRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			Amount: command.Amount, Currency: command.Currency,
		}
	case payment.CommandCreditCommission:
		topic, messageType, step = c.topics.SettlementCommands, messagebus.CreditCommissionRequestedType, messagebus.StepSettlement
		payload = messagebus.CreditCommissionRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			TransactionAmount: command.Amount, Rank: command.Rank,
		}
	case payment.CommandDispatchCallback:
		topic, messageType, step = c.topics.AgentCommands, messagebus.DispatchCallbackRequestedType, messagebus.StepCallback
		payload = messagebus.DispatchCallbackRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			Amount: command.Amount, Currency: command.Currency,
		}
	default:
		return EncodedCommand{}, fmt.Errorf("payment codec: unsupported command %q", command.Kind)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return EncodedCommand{}, fmt.Errorf("payment codec: marshal %s: %w", command.Kind, err)
	}
	return EncodedCommand{
		Topic: topic, Key: command.TransactionID,
		Message: &platformkafka.Message{
			ID: messagebus.MessageID(command.TransactionID, step), EventType: messageType,
			AggregateType: "PaymentSaga", AggregateID: command.TransactionID, Payload: encoded,
		},
	}, nil
}

func (c *Kafka) TopicFor(eventType string) string {
	switch eventType {
	case messagebus.PaymentInitiatedType:
		return c.topics.PaymentInitiatedEvents
	case messagebus.PaymentCompletedType:
		return c.topics.PaymentEvents
	case messagebus.VolumeAccumulatedType:
		return c.topics.AffiliateEvents
	case messagebus.CommissionCreditedType:
		return c.topics.SettlementEvents
	default:
		return c.topics.AgentEvents
	}
}

func decode(message *platformkafka.Message, target any) error {
	if err := json.Unmarshal(message.Payload, target); err != nil {
		return processcore.Invalid("decode %s: %v", message.EventType, err)
	}
	return nil
}
