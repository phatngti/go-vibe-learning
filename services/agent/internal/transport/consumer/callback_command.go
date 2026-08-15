package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"
)

// Keep the existing Kafka group/inbox consumer identity during the worker
// refactor so deploying the durable scheduler does not replay retained history.
const callbackConsumerName = "agent-service"

// CallbackCommandHandler atomically checkpoints the Kafka command and its
// durable delivery intent. It never performs the callback network call.
type CallbackCommandHandler struct {
	agents          port.AgentRepository
	deliveries      port.CallbackDeliveryRepository
	requireVerified bool
	commandTopic    string
	eventTopic      string
	now             func() time.Time
}

func NewCallbackCommandHandler(
	agents port.AgentRepository,
	deliveries port.CallbackDeliveryRepository,
	cfg *config.Config,
) *CallbackCommandHandler {
	return &CallbackCommandHandler{
		agents: agents, deliveries: deliveries, requireVerified: cfg.Callback.RequireVerified,
		commandTopic: cfg.Kafka.Topics.AgentCommands, eventTopic: cfg.Kafka.Topics.AgentEvents,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func NewCallbackSubscription(handler *CallbackCommandHandler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AgentCommands, GroupID: callbackConsumerName, Handler: handler.Handle}
}

func (h *CallbackCommandHandler) Handle(ctx context.Context, msg *kafka.Message) error {
	if msg.EventType != messagebus.DispatchCallbackRequestedType {
		return kafka.Permanent(fmt.Errorf("agent: unsupported command %q", msg.EventType))
	}
	var payload messagebus.DispatchCallbackRequested
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return kafka.Permanent(fmt.Errorf("agent: decode callback command: %w", err))
	}
	monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("agent: invalid callback money: %w", err))
	}
	if payload.TransactionID == "" || payload.AgentID == "" || !monetaryAmount.IsPositive() {
		return kafka.Permanent(fmt.Errorf("agent: invalid callback command payload"))
	}
	legacyAmount, err := messagebus.LegacyMajorFloat(monetaryAmount)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("agent: render callback compatibility amount: %w", err))
	}

	msg.ID = messagebus.MessageID(payload.TransactionID, messagebus.StepCallback)
	delivery := &domain.CallbackDelivery{
		TransactionID: payload.TransactionID, AgentID: payload.AgentID, EventID: msg.ID,
		Status: domain.CallbackDeliveryPending, OutcomeGeneration: 1,
	}
	agentCfg, err := h.agents.GetConfig(ctx, payload.AgentID)
	if err != nil {
		return err
	}
	if agentCfg.CallbackURL == "" {
		return h.scheduleTerminal(ctx, msg, delivery, messagebus.CallbackDispatchSkippedType,
			domain.CallbackDeliverySkipped, "callback URL is not configured")
	}
	delivery.TenantID = agentCfg.TenantID
	delivery.CallbackURL = agentCfg.CallbackURL
	if h.requireVerified && (agentCfg.CallbackStatus != domain.CallbackStatusVerified || agentCfg.CallbackVerifiedAt == nil || agentCfg.TenantID == "") {
		return h.scheduleTerminal(ctx, msg, delivery, messagebus.CallbackDispatchFailedType,
			domain.CallbackDeliveryTerminalFailed, domain.ErrCallbackNotVerified.Error())
	}

	body, err := json.Marshal(map[string]any{
		"event_id": msg.ID, "tenant_id": agentCfg.TenantID,
		"transaction_id": payload.TransactionID, "agent_id": payload.AgentID,
		"money": monetaryAmount, "amount": legacyAmount, "currency": monetaryAmount.Currency,
		"status": "COMPLETED",
	})
	if err != nil {
		return kafka.Permanent(err)
	}
	now := h.now()
	delivery.Payload = body
	delivery.NextAttemptAt = &now
	_, _, err = h.deliveries.Schedule(ctx, port.ScheduleCallbackParams{
		ConsumerName: callbackConsumerName, MessageID: msg.ID, Topic: h.commandTopic,
		MessageType: msg.EventType, Delivery: delivery,
	})
	return err
}

func (h *CallbackCommandHandler) scheduleTerminal(
	ctx context.Context,
	msg *kafka.Message,
	delivery *domain.CallbackDelivery,
	eventType, status, reason string,
) error {
	now := h.now()
	delivery.Status = status
	delivery.LastError = reason
	delivery.CreatedAt = now
	delivery.UpdatedAt = now
	if status == domain.CallbackDeliveryTerminalFailed {
		delivery.TerminalAt = &now
	}
	result, err := callbackResultIntent(delivery, h.eventTopic, eventType, status, reason)
	if err != nil {
		return kafka.Permanent(err)
	}
	_, _, err = h.deliveries.Schedule(ctx, port.ScheduleCallbackParams{
		ConsumerName: callbackConsumerName, MessageID: msg.ID, Topic: h.commandTopic,
		MessageType: msg.EventType, Delivery: delivery, Result: &result,
	})
	return err
}

func callbackResultIntent(
	delivery *domain.CallbackDelivery,
	eventTopic, eventType, status, reason string,
) (port.CallbackResultIntent, error) {
	payload, err := json.Marshal(messagebus.CallbackResult{
		TransactionID: delivery.TransactionID, AgentID: delivery.AgentID, Status: status, Reason: reason,
	})
	if err != nil {
		return port.CallbackResultIntent{}, err
	}
	generation := delivery.OutcomeGeneration
	if generation == 0 {
		generation = 1
	}
	return port.CallbackResultIntent{
		MessageID: messagebus.MessageID(delivery.TransactionID, fmt.Sprintf("callback-result:g%d", generation)),
		Topic:     eventTopic, MessageKey: delivery.TransactionID, MessageType: eventType,
		AggregateType: "CallbackDelivery", AggregateID: delivery.TransactionID,
		TenantID: delivery.TenantID, CausationID: delivery.EventID, Payload: payload,
	}, nil
}
