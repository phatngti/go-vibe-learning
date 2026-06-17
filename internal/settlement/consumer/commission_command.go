package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/settlement/application/command"
	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/domain"

	"gorm.io/gorm"
)

type CommissionCommandHandler struct {
	db      *gorm.DB
	inbox   inbox.Repository
	handler commissionCreditor
	topic   string
}

type commissionCreditor interface {
	HandleInTx(context.Context, *gorm.DB, *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error)
}

func NewCommissionCommandHandler(db *platformpostgres.DB, inboxRepo inbox.Repository, handler *command.CalculateAndCreditCommissionHandler, cfg *config.Config) *CommissionCommandHandler {
	return &CommissionCommandHandler{db: db.DB, inbox: inboxRepo, handler: handler, topic: cfg.Kafka.Topics.SettlementCommands}
}

func NewCommissionSubscription(handler *CommissionCommandHandler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.SettlementCommands, GroupID: "settlement-service", Handler: handler.Handle}
}

func (h *CommissionCommandHandler) Handle(ctx context.Context, msg *kafka.Message) error {
	if msg.EventType != messagebus.CreditCommissionRequestedType {
		return kafka.Permanent(fmt.Errorf("settlement: unsupported command %q", msg.EventType))
	}
	var payload messagebus.CreditCommissionRequested
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return kafka.Permanent(fmt.Errorf("settlement: decode commission command: %w", err))
	}
	if payload.TransactionID == "" || payload.AgentID == "" || payload.TransactionAmount <= 0 || payload.Rank == "" {
		return kafka.Permanent(fmt.Errorf("settlement: invalid commission command payload"))
	}
	dedupID := messagebus.MessageID(payload.TransactionID, messagebus.StepSettlement)
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, dedupID, h.topic, msg.EventType)
		if err != nil || !fresh {
			return err
		}
		if _, _, err := h.handler.HandleInTx(ctx, tx, &dto.CalculateAndCreditCommission{
			TransactionID:     payload.TransactionID,
			AgentID:           payload.AgentID,
			TransactionAmount: payload.TransactionAmount,
			Rank:              payload.Rank,
		}); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, dedupID)
	})
}
