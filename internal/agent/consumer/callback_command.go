package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github/master_go_cqrs/internal/agent/domain"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

const maxCallbackAttempts = 5

type CallbackCommandHandler struct {
	db           *gorm.DB
	repo         domain.AgentRepository
	inbox        inbox.Repository
	outbox       outbox.Repository
	httpClient   *http.Client
	commandTopic string
	eventTopic   string
}

func NewCallbackCommandHandler(db *platformpostgres.DB, repo domain.AgentRepository, inboxRepo inbox.Repository, outboxRepo outbox.Repository, cfg *config.Config) *CallbackCommandHandler {
	return &CallbackCommandHandler{
		db: db.DB, repo: repo, inbox: inboxRepo, outbox: outboxRepo,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		commandTopic: cfg.Kafka.Topics.AgentCommands,
		eventTopic:   cfg.Kafka.Topics.AgentEvents,
	}
}

func NewCallbackSubscription(handler *CallbackCommandHandler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AgentCommands, GroupID: "agent-service", Handler: handler.Handle}
}

func (h *CallbackCommandHandler) Handle(ctx context.Context, msg *kafka.Message) error {
	if msg.EventType != messagebus.DispatchCallbackRequestedType {
		return kafka.Permanent(fmt.Errorf("agent: unsupported command %q", msg.EventType))
	}
	var payload messagebus.DispatchCallbackRequested
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return kafka.Permanent(fmt.Errorf("agent: decode callback command: %w", err))
	}
	if payload.TransactionID == "" || payload.AgentID == "" {
		return kafka.Permanent(fmt.Errorf("agent: invalid callback command payload"))
	}
	msg.ID = messagebus.MessageID(payload.TransactionID, messagebus.StepCallback)

	delivery, err := h.prepare(ctx, msg, payload)
	if err != nil {
		return err
	}
	if delivery.Status == "DELIVERED" || delivery.Status == "SKIPPED" || delivery.Status == "FAILED" {
		return nil
	}

	config, err := h.repo.GetConfig(ctx, payload.AgentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchSkippedType, "SKIPPED", "agent callback configuration was not found")
		}
		return err
	}
	if config.CallbackURL == "" {
		return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchSkippedType, "SKIPPED", "callback URL is not configured")
	}
	delivery.CallbackURL = config.CallbackURL

	body, err := json.Marshal(map[string]any{
		"transaction_id": payload.TransactionID,
		"agent_id":       payload.AgentID,
		"amount":         payload.Amount,
		"currency":       payload.Currency,
		"status":         "COMPLETED",
	})
	if err != nil {
		return kafka.Permanent(err)
	}
	status, err := h.dispatch(ctx, config.CallbackURL, payload.TransactionID, body)
	delivery.Attempts++
	if err != nil {
		if kafka.IsPermanent(err) {
			return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchFailedType, "FAILED", err.Error())
		}
		delivery.LastError = err.Error()
		if saveErr := h.repo.SaveCallbackDelivery(ctx, nil, delivery); saveErr != nil {
			return saveErr
		}
		if delivery.Attempts >= maxCallbackAttempts {
			return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchFailedType, "FAILED", err.Error())
		}
		return err
	}
	if status >= http.StatusBadRequest {
		reason := fmt.Sprintf("callback returned HTTP %d", status)
		if status >= http.StatusInternalServerError && delivery.Attempts < maxCallbackAttempts {
			delivery.LastError = reason
			if err := h.repo.SaveCallbackDelivery(ctx, nil, delivery); err != nil {
				return err
			}
			return errors.New(reason)
		}
		return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchFailedType, "FAILED", reason)
	}
	return h.finalize(ctx, msg, delivery, messagebus.CallbackDispatchedType, "DELIVERED", "")
}

func (h *CallbackCommandHandler) prepare(ctx context.Context, msg *kafka.Message, payload messagebus.DispatchCallbackRequested) (*domain.CallbackDelivery, error) {
	delivery, err := h.repo.GetCallbackDelivery(ctx, payload.TransactionID)
	if err != nil {
		return nil, err
	}
	if delivery != nil {
		return delivery, nil
	}
	delivery = &domain.CallbackDelivery{TransactionID: payload.TransactionID, AgentID: payload.AgentID, Status: "PENDING"}
	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, msg.ID, h.commandTopic, msg.EventType)
		if err != nil || !fresh {
			return err
		}
		return h.repo.SaveCallbackDelivery(ctx, tx, delivery)
	})
	return delivery, err
}

func (h *CallbackCommandHandler) dispatch(ctx context.Context, callbackURL, transactionID string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return 0, kafka.Permanent(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", transactionID)
	response, err := h.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}

func (h *CallbackCommandHandler) finalize(ctx context.Context, msg *kafka.Message, delivery *domain.CallbackDelivery, eventType, status, reason string) error {
	delivery.Status = status
	delivery.LastError = reason
	payload, err := json.Marshal(messagebus.CallbackResult{
		TransactionID: delivery.TransactionID, AgentID: delivery.AgentID,
		Status: status, Reason: reason,
	})
	if err != nil {
		return err
	}
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := h.repo.SaveCallbackDelivery(ctx, tx, delivery); err != nil {
			return err
		}
		if err := h.outbox.Enqueue(ctx, tx, outbox.EnqueueParams{
			MessageID: messagebus.MessageID(delivery.TransactionID, "callback-result"),
			Topic:     h.eventTopic, MessageKey: delivery.TransactionID,
			MessageType: eventType, AggregateType: "CallbackDelivery",
			AggregateID: delivery.TransactionID, Payload: payload,
		}); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, msg.ID)
	})
}
