package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/internal/affiliate/application/command"
	"github/master_go_cqrs/internal/affiliate/application/dto"
	"github/master_go_cqrs/internal/affiliate/domain"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

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
	if payload.TransactionID == "" || payload.AgentID == "" || payload.Amount <= 0 {
		return kafka.Permanent(fmt.Errorf("affiliate: invalid volume command payload"))
	}
	dedupID := messagebus.MessageID(payload.TransactionID, messagebus.StepAffiliate)
	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := h.inbox.Begin(ctx, tx, dedupID, h.topic, msg.EventType)
		if err != nil || !fresh {
			return err
		}
		if _, _, _, err := h.handler.HandleInTx(ctx, tx, &dto.AccumulateVolume{
			TransactionID: payload.TransactionID,
			AgentID:       payload.AgentID,
			Amount:        payload.Amount,
		}); err != nil {
			return err
		}
		return h.inbox.MarkProcessed(ctx, tx, dedupID)
	})
}
