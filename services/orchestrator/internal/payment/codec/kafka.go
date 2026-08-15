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
	topics                  config.KafkaTopics
	orchestrationOwner      string
	orchestrationGeneration int64
}

func NewKafka(cfg *config.Config) *Kafka {
	owner, generation := cfg.OrchestrationIdentity()
	return &Kafka{topics: cfg.Kafka.Topics, orchestrationOwner: owner, orchestrationGeneration: generation}
}

func (c *Kafka) Decode(message *platformkafka.Message) (payment.Event, error) {
	var event payment.Event
	switch message.EventType {
	case messagebus.PaymentInitiatedType:
		var payload messagebus.PaymentInitiated
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
		if err != nil {
			return event, processcore.Invalid("decode %s money: %v", message.EventType, err)
		}
		event = payment.Event{
			Kind: payment.EventPaymentInitiated, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Money: messagebus.MoneyPointer(monetaryAmount), Amount: payload.Amount, Currency: monetaryAmount.Currency,
		}
	case messagebus.PaymentCompletedType:
		var payload messagebus.PaymentCompleted
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
		if err != nil {
			return event, processcore.Invalid("decode %s money: %v", message.EventType, err)
		}
		event = payment.Event{
			Kind: payment.EventPaymentCompleted, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Money: messagebus.MoneyPointer(monetaryAmount), Amount: payload.Amount, Currency: monetaryAmount.Currency,
		}
	case messagebus.PaymentFailedType:
		var payload messagebus.PaymentFailed
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
		if err != nil {
			return event, processcore.Invalid("decode %s money: %v", message.EventType, err)
		}
		event = payment.Event{
			Kind: payment.EventPaymentFailed, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Money: messagebus.MoneyPointer(monetaryAmount), Amount: payload.Amount, Currency: monetaryAmount.Currency, Reason: payload.Reason,
		}
	case messagebus.PaymentProcessingUncertainType:
		var payload messagebus.PaymentProcessingUncertain
		if err := decode(message, &payload); err != nil {
			return event, err
		}
		monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
		if err != nil {
			return event, processcore.Invalid("decode %s money: %v", message.EventType, err)
		}
		event = payment.Event{
			Kind: payment.EventPaymentUncertain, TransactionID: payload.TransactionID,
			AgentID: payload.AgentID, Money: messagebus.MoneyPointer(monetaryAmount), Amount: payload.Amount, Currency: monetaryAmount.Currency, Reason: payload.Reason,
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
		if payload.Money != nil {
			monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Money.Currency)
			if err != nil {
				return event, processcore.Invalid("decode %s money: %v", message.EventType, err)
			}
			event.Money = messagebus.MoneyPointer(monetaryAmount)
			event.Currency = monetaryAmount.Currency
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
	event.MessageID = message.ID
	event.OrchestrationOwner = message.OrchestrationOwner
	event.OrchestrationGeneration = message.OrchestrationGeneration
	if event.OrchestrationOwner == "" && event.OrchestrationGeneration == 0 {
		event.OrchestrationOwner = c.orchestrationOwner
		event.OrchestrationGeneration = c.orchestrationGeneration
	}
	if err := payment.ValidateEvent(event); err != nil {
		return payment.Event{}, err
	}
	return event, nil
}

func (c *Kafka) Encode(command payment.Command) (EncodedCommand, error) {
	owner := command.OrchestrationOwner
	generation := command.OrchestrationGeneration
	if owner == "" && generation == 0 {
		owner, generation = c.orchestrationOwner, c.orchestrationGeneration
	}
	if owner == "" || generation < 1 {
		return EncodedCommand{}, fmt.Errorf("payment codec: orchestration owner and positive generation are required")
	}
	if c.orchestrationOwner != "" && (owner != c.orchestrationOwner || generation != c.orchestrationGeneration) {
		return EncodedCommand{}, fmt.Errorf("payment codec: command owner %s generation %d does not match runtime %s generation %d",
			owner, generation, c.orchestrationOwner, c.orchestrationGeneration)
	}
	monetaryAmount, err := command.MonetaryAmount()
	if err != nil {
		return EncodedCommand{}, fmt.Errorf("payment codec: invalid command money: %w", err)
	}
	if !monetaryAmount.IsPositive() {
		return EncodedCommand{}, fmt.Errorf("payment codec: command money must be positive")
	}
	legacyAmount, err := messagebus.LegacyMajorFloat(monetaryAmount)
	if err != nil {
		return EncodedCommand{}, fmt.Errorf("payment codec: render legacy command money: %w", err)
	}
	var topic, messageType, step string
	var payload any
	switch command.Kind {
	case payment.CommandAccumulateVolume:
		topic, messageType, step = c.topics.AffiliateCommands, messagebus.AccumulateVolumeRequestedType, messagebus.StepAffiliate
		payload = messagebus.AccumulateVolumeRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			Money: messagebus.MoneyPointer(monetaryAmount), Amount: legacyAmount, Currency: monetaryAmount.Currency,
		}
	case payment.CommandCreditCommission:
		topic, messageType, step = c.topics.SettlementCommands, messagebus.CreditCommissionRequestedType, messagebus.StepSettlement
		payload = messagebus.CreditCommissionRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			TransactionMoney: messagebus.MoneyPointer(monetaryAmount), TransactionAmount: legacyAmount,
			Currency: monetaryAmount.Currency, Rank: command.Rank,
		}
	case payment.CommandDispatchCallback:
		topic, messageType, step = c.topics.AgentCommands, messagebus.DispatchCallbackRequestedType, messagebus.StepCallback
		payload = messagebus.DispatchCallbackRequested{
			TransactionID: command.TransactionID, AgentID: command.AgentID,
			Money: messagebus.MoneyPointer(monetaryAmount), Amount: legacyAmount, Currency: monetaryAmount.Currency,
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
			ID: orchestrationMessageID(command.TransactionID, step, generation), EventType: messageType,
			AggregateType: "PaymentSaga", AggregateID: command.TransactionID, Payload: encoded,
			SchemaVersion: 1, CorrelationID: command.TransactionID,
			OrchestrationOwner: owner, OrchestrationGeneration: generation,
		},
	}, nil
}

func orchestrationMessageID(transactionID, step string, generation int64) string {
	base := messagebus.MessageID(transactionID, step)
	if generation <= 1 {
		return base
	}
	return fmt.Sprintf("%s:g%d", base, generation)
}

func (c *Kafka) TopicFor(eventType string) string {
	switch eventType {
	case messagebus.PaymentInitiatedType:
		return c.topics.PaymentInitiatedEvents
	case messagebus.PaymentCompletedType, messagebus.PaymentFailedType, messagebus.PaymentProcessingUncertainType:
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
