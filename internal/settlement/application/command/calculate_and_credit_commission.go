package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	"github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/settlement/application/dto"
	"github/master_go_cqrs/internal/settlement/domain"

	"gorm.io/gorm"
)

type CalculateAndCreditCommissionHandler struct {
	db         *gorm.DB
	repo       domain.SettlementRepository
	eventStore eventstore.Repository
	outbox     outbox.Repository
	eventTopic string
}

func NewCalculateAndCreditCommissionHandler(db *postgres.DB, repo domain.SettlementRepository, es eventstore.Repository, outboxRepo outbox.Repository, cfg *config.Config) *CalculateAndCreditCommissionHandler {
	return &CalculateAndCreditCommissionHandler{
		db: db.DB, repo: repo, eventStore: es, outbox: outboxRepo,
		eventTopic: cfg.Kafka.Topics.SettlementEvents,
	}
}

func (h *CalculateAndCreditCommissionHandler) Handle(ctx context.Context, cmd *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	var balance *domain.AgentBalance
	var commission float64
	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		var err error
		balance, commission, err = h.HandleInTx(ctx, dbTx, cmd)
		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to calculate and credit commission: %w", err)
	}
	return balance, commission, nil
}

func (h *CalculateAndCreditCommissionHandler) HandleInTx(ctx context.Context, dbTx *gorm.DB, cmd *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	rate := 0.01
	switch cmd.Rank {
	case "SILVER":
		rate = 0.02
	case "GOLD":
		rate = 0.03
	case "PLATINUM":
		rate = 0.04
	case "DIAMOND":
		rate = 0.05
	}

	commission := cmd.TransactionAmount * rate
	var updatedBalance *domain.AgentBalance

	err := func() error {
		bal, err := h.repo.GetBalanceForUpdate(ctx, dbTx, cmd.AgentID)
		if err != nil {
			return err
		}

		bal.ActiveBalance += commission
		if err := h.repo.SaveBalance(ctx, dbTx, bal); err != nil {
			return err
		}

		log := &domain.CommissionLog{
			AgentID:           cmd.AgentID,
			TransactionID:     cmd.TransactionID,
			TransactionAmount: cmd.TransactionAmount,
			CommissionAmount:  commission,
			Rank:              cmd.Rank,
		}
		if err := h.repo.CreateCommissionLog(ctx, dbTx, log); err != nil {
			return err
		}

		updatedBalance = bal

		// Append CommissionCredited domain event
		evt := domain.CommissionCredited{
			AgentID:           cmd.AgentID,
			TransactionID:     cmd.TransactionID,
			TransactionAmount: cmd.TransactionAmount,
			CommissionAmount:  commission,
			Rank:              cmd.Rank,
			NewBalance:        bal.ActiveBalance,
			OccurredAt:        time.Now(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("calculate_commission: marshal event: %w", err)
		}
		version, err := h.eventStore.NextVersion(ctx, dbTx, "AgentBalance", cmd.AgentID)
		if err != nil {
			return err
		}
		if err := h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
			AggregateType: "AgentBalance",
			AggregateID:   cmd.AgentID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		}); err != nil {
			return err
		}
		return h.outbox.Enqueue(ctx, dbTx, outbox.EnqueueParams{
			MessageID: messagebus.MessageID(cmd.TransactionID, "commission-credited"),
			Topic:     h.eventTopic, MessageKey: cmd.TransactionID,
			MessageType:   messagebus.CommissionCreditedType,
			AggregateType: "AgentBalance", AggregateID: cmd.AgentID,
			Payload: payload,
		})
	}()

	if err != nil {
		return nil, 0, err
	}

	return updatedBalance, commission, nil
}
