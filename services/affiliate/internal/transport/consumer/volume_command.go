package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/affiliate/internal/application/command"
	"github/master_go_cqrs/services/affiliate/internal/application/dto"
	"github/master_go_cqrs/services/affiliate/internal/config"
	"github/master_go_cqrs/services/affiliate/internal/domain"

	"gorm.io/gorm"
)

type VolumeCommandHandler struct {
	db      *gorm.DB
	inbox   inbox.Repository
	handler volumeAccumulator
	topic   string
}

type volumeAccumulator interface {
	HandleInTx(context.Context, *gorm.DB, *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error)
}

func NewVolumeCommandHandler(db *platformpostgres.DB, inboxRepo inbox.Repository, handler *command.AccumulateVolumeHandler, cfg *config.Config) *VolumeCommandHandler {
	return &VolumeCommandHandler{db: db.DB, inbox: inboxRepo, handler: handler, topic: cfg.Kafka.Topics.AffiliateCommands}
}

func NewVolumeSubscription(handler *VolumeCommandHandler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AffiliateCommands, GroupID: "affiliate-service", Handler: handler.Handle}
}

func (h *VolumeCommandHandler) Handle(ctx context.Context, msg *kafka.Message) error {
	if msg.EventType != messagebus.AccumulateVolumeRequestedType {
		return kafka.Permanent(fmt.Errorf("affiliate: unsupported command %q", msg.EventType))
	}
	var payload messagebus.AccumulateVolumeRequested
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return kafka.Permanent(fmt.Errorf("affiliate: decode volume command: %w", err))
	}
	monetaryAmount, err := messagebus.ResolveMoney(payload.Money, payload.Amount, payload.Currency)
	if err != nil {
		return kafka.Permanent(fmt.Errorf("affiliate: invalid volume money: %w", err))
	}
	if payload.TransactionID == "" || payload.AgentID == "" || !monetaryAmount.IsPositive() {
		return kafka.Permanent(fmt.Errorf("affiliate: invalid volume command payload"))
	}
	dedupID := messagebus.MessageID(payload.TransactionID, messagebus.StepAffiliate)
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, "affiliate-service", dedupID, h.topic, msg.EventType)
		if err != nil || !fresh {
			return err
		}
		// Propagate the transaction through context so repository adapters can extract it.
		txCtx := platformpostgres.WithTx(ctx, tx)
		if _, _, _, err := h.handler.HandleInTx(txCtx, tx, &dto.AccumulateVolume{
			TransactionID: payload.TransactionID,
			AgentID:       payload.AgentID,
			Money:         messagebus.MoneyPointer(monetaryAmount),
			Amount:        payload.Amount,
			Currency:      payload.Currency,
		}); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, "affiliate-service", dedupID)
	})
}
