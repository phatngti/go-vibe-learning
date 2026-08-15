package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/outbox"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/affiliate/internal/application/dto"
	"github/master_go_cqrs/services/affiliate/internal/application/port"
	"github/master_go_cqrs/services/affiliate/internal/config"
	"github/master_go_cqrs/services/affiliate/internal/domain"

	"gorm.io/gorm"
)

type AccumulateVolumeHandler struct {
	uow        port.UnitOfWork
	repo       port.AffiliateRepository
	eventStore eventstore.Repository
	outbox     outbox.Repository
	eventTopic string
}

func NewAccumulateVolumeHandler(uow port.UnitOfWork, repo port.AffiliateRepository, es eventstore.Repository, outboxRepo outbox.Repository, cfg *config.Config) *AccumulateVolumeHandler {
	return &AccumulateVolumeHandler{
		uow: uow, repo: repo, eventStore: es, outbox: outboxRepo,
		eventTopic: cfg.Kafka.Topics.AffiliateEvents,
	}
}

func (h *AccumulateVolumeHandler) Handle(ctx context.Context, cmd *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	var metrics *domain.AgentMetric
	var finalRank domain.Rank
	var promoted bool
	err := h.uow.Do(ctx, func(txCtx context.Context) error {
		var err error
		metrics, finalRank, promoted, err = h.HandleInTx(txCtx, nil, cmd)
		return err
	})
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to accumulate volume: %w", err)
	}
	return metrics, finalRank, promoted, nil
}

// HandleInTx performs the volume accumulation within a transaction.
// When called from a consumer, dbTx is the raw *gorm.DB transaction for
// event store and outbox operations (platform-layer concern). Repository
// operations use the txCtx which carries the transaction via context.
func (h *AccumulateVolumeHandler) HandleInTx(ctx context.Context, dbTx *gorm.DB, cmd *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	var metrics *domain.AgentMetric
	var finalRank domain.Rank
	var promoted bool
	var oldRank domain.Rank

	err := func() error {
		incomingMoney, err := messagebus.ResolveMoney(cmd.Money, cmd.Amount, cmd.Currency)
		if err != nil {
			return fmt.Errorf("accumulate_volume: invalid money: %w", err)
		}
		if !incomingMoney.IsPositive() {
			return fmt.Errorf("accumulate_volume: amount must be positive")
		}

		metrics, err = h.repo.GetMetricsForUpdate(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		currentMoney, err := metricMoney(metrics, incomingMoney)
		if err != nil {
			return err
		}
		totalMoney, err := currentMoney.Add(incomingMoney)
		if err != nil {
			return fmt.Errorf("accumulate_volume: add volume: %w", err)
		}
		legacyAmount, err := messagebus.LegacyMajorFloat(incomingMoney)
		if err != nil {
			return fmt.Errorf("accumulate_volume: render legacy amount: %w", err)
		}
		legacyTotal, err := messagebus.LegacyMajorFloat(totalMoney)
		if err != nil {
			return fmt.Errorf("accumulate_volume: render legacy total: %w", err)
		}
		metrics.AccumulatedVolume = legacyTotal
		metrics.AccumulatedVolumeMoney = messagebus.MoneyPointer(totalMoney)
		if err := h.repo.SaveMetrics(ctx, metrics); err != nil {
			return err
		}

		level, err := h.repo.GetLevelForUpdate(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		oldRank = level.CurrentRank
		newRank, err := evaluateRankMoney(totalMoney)
		if err != nil {
			return err
		}
		finalRank = newRank

		if newRank != oldRank {
			promoted = true
			level.CurrentRank = newRank
			if err := h.repo.SaveLevel(ctx, level); err != nil {
				return err
			}
		}

		now := time.Now()

		// Append VolumeAccumulated event
		volEvt := domain.VolumeAccumulated{
			TransactionID: cmd.TransactionID,
			AgentID:       cmd.AgentID,
			Money:         messagebus.MoneyPointer(incomingMoney),
			TotalMoney:    messagebus.MoneyPointer(totalMoney),
			Amount:        legacyAmount,
			TotalVolume:   legacyTotal,
			CurrentRank:   finalRank,
			RankPromoted:  promoted,
			OccurredAt:    now,
		}
		volPayload, err := json.Marshal(volEvt)
		if err != nil {
			return fmt.Errorf("accumulate_volume: marshal VolumeAccumulated: %w", err)
		}
		version, err := h.eventStore.NextVersion(ctx, dbTx, "AgentMetric", cmd.AgentID)
		if err != nil {
			return err
		}
		if err := h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
			AggregateType: "AgentMetric",
			AggregateID:   cmd.AgentID,
			EventType:     volEvt.EventType(),
			Payload:       string(volPayload),
			Version:       version,
		}); err != nil {
			return err
		}
		if cmd.TransactionID != "" {
			if err := h.outbox.Enqueue(ctx, dbTx, outbox.EnqueueParams{
				MessageID: messagebus.MessageID(cmd.TransactionID, "volume-accumulated"),
				Topic:     h.eventTopic, MessageKey: cmd.TransactionID,
				MessageType:   messagebus.VolumeAccumulatedType,
				AggregateType: "AgentMetric", AggregateID: cmd.AgentID,
				Payload: volPayload,
			}); err != nil {
				return err
			}
		}

		// Append RankPromoted event if rank changed
		if promoted {
			rankEvt := domain.RankPromoted{
				AgentID:    cmd.AgentID,
				OldRank:    oldRank,
				NewRank:    finalRank,
				OccurredAt: now,
			}
			rankPayload, err := json.Marshal(rankEvt)
			if err != nil {
				return fmt.Errorf("accumulate_volume: marshal RankPromoted: %w", err)
			}
			rankVersion, err := h.eventStore.NextVersion(ctx, dbTx, "AffiliateLevel", cmd.AgentID)
			if err != nil {
				return err
			}
			if err := h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
				AggregateType: "AffiliateLevel",
				AggregateID:   cmd.AgentID,
				EventType:     rankEvt.EventType(),
				Payload:       string(rankPayload),
				Version:       rankVersion,
			}); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		return nil, "", false, err
	}

	return metrics, finalRank, promoted, nil
}

func metricMoney(metrics *domain.AgentMetric, incoming sharedmoney.Money) (sharedmoney.Money, error) {
	if metrics.AccumulatedVolumeMoney != nil {
		value, err := sharedmoney.New(
			metrics.AccumulatedVolumeMoney.AmountMinor,
			metrics.AccumulatedVolumeMoney.Currency,
			metrics.AccumulatedVolumeMoney.Exponent,
		)
		if err != nil {
			return sharedmoney.Money{}, fmt.Errorf("accumulate_volume: stored money is invalid: %w", err)
		}
		return value, nil
	}
	if metrics.AccumulatedVolume != 0 {
		return sharedmoney.Money{}, domain.ErrMoneyBackfillRequired
	}
	return sharedmoney.New(0, incoming.Currency, incoming.Exponent)
}

func evaluateRankMoney(volume sharedmoney.Money) (domain.Rank, error) {
	if err := volume.Validate(); err != nil {
		return "", fmt.Errorf("evaluate affiliate rank: %w", err)
	}
	scale := int64(1)
	for i := int32(0); i < volume.Exponent; i++ {
		scale *= 10
	}
	if volume.AmountMinor >= 1_000_000*scale {
		return domain.RankDiamond, nil
	} else if volume.AmountMinor >= 100_000*scale {
		return domain.RankPlatinum, nil
	} else if volume.AmountMinor >= 10_000*scale {
		return domain.RankGold, nil
	} else if volume.AmountMinor >= 1_000*scale {
		return domain.RankSilver, nil
	}
	return domain.RankBronze, nil
}

// evaluateRank is kept for legacy tests and v1 compatibility adapters. New
// command processing uses evaluateRankMoney and never performs float arithmetic.
func evaluateRank(volume float64) domain.Rank {
	value, err := sharedmoney.FromLegacyFloat64(volume, "USD")
	if err != nil {
		return domain.RankBronze
	}
	rank, err := evaluateRankMoney(value)
	if err != nil {
		return domain.RankBronze
	}
	return rank
}
