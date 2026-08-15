package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/settlement/internal/application/command"
	"github/master_go_cqrs/services/settlement/internal/application/dto"
	"github/master_go_cqrs/services/settlement/internal/config"
	"github/master_go_cqrs/services/settlement/internal/domain"

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
	legacyCurrency := payload.Currency
	if payload.TransactionMoney == nil && legacyCurrency == "" {
		// Commands emitted before Money v2 did not carry currency. USD is the
		// explicit v1 compatibility default and must be removed with v1 topics.
		legacyCurrency = "USD"
	}
	transactionMoney, err := messagebus.ResolveMoney(payload.TransactionMoney, payload.TransactionAmount, legacyCurrency)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("settlement: invalid commission money: %w", err))
	}
	if payload.TransactionID == "" || payload.AgentID == "" || !transactionMoney.IsPositive() || !validRank(payload.Rank) {
		return kafka.Permanent(fmt.Errorf("settlement: invalid commission command payload"))
	}
	dedupID := messagebus.MessageID(payload.TransactionID, messagebus.StepSettlement)
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, "settlement-service", dedupID, h.topic, msg.EventType)
		if err != nil || !fresh {
			return err
		}
		txCtx := platformpostgres.WithTx(ctx, tx)
		if _, _, err := h.handler.HandleInTx(txCtx, tx, &dto.CalculateAndCreditCommission{
			TransactionID:     payload.TransactionID,
			AgentID:           payload.AgentID,
			TransactionMoney:  messagebus.MoneyPointer(transactionMoney),
			TransactionAmount: payload.TransactionAmount,
			Currency:          legacyCurrency,
			Rank:              payload.Rank,
		}); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, "settlement-service", dedupID)
	})
}

func validRank(rank string) bool {
	switch rank {
	case "BRONZE", "SILVER", "GOLD", "PLATINUM", "DIAMOND":
		return true
	default:
		return false
	}
}
