package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/affiliate/application/dto"
	"github/master_go_cqrs/internal/affiliate/domain"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type AccumulateVolumeHandler struct {
	db         *gorm.DB
	repo       domain.AffiliateRepository
	eventStore eventstore.Repository
	outbox     outbox.Repository
	eventTopic string
}

func NewAccumulateVolumeHandler(db *platformpostgres.DB, repo domain.AffiliateRepository, es eventstore.Repository, outboxRepo outbox.Repository, cfg *config.Config) *AccumulateVolumeHandler {
	return &AccumulateVolumeHandler{
		db: db.DB, repo: repo, eventStore: es, outbox: outboxRepo,
		eventTopic: cfg.Kafka.Topics.AffiliateEvents,
	}
}

func (h *AccumulateVolumeHandler) Handle(ctx context.Context, cmd *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	var metrics *domain.AgentMetric
	var finalRank domain.Rank
	var promoted bool
	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		var err error
		metrics, finalRank, promoted, err = h.HandleInTx(ctx, dbTx, cmd)
		return err
	})
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to accumulate volume: %w", err)
	}
	return metrics, finalRank, promoted, nil
}

func (h *AccumulateVolumeHandler) HandleInTx(ctx context.Context, dbTx *gorm.DB, cmd *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	var metrics *domain.AgentMetric
	var finalRank domain.Rank
	var promoted bool
	var oldRank domain.Rank

	err := func() error {
		var err error
		metrics, err = h.repo.GetMetricsForUpdate(ctx, dbTx, cmd.AgentID)
		if err != nil {
			return err
		}

		metrics.AccumulatedVolume += cmd.Amount
		if err := h.repo.SaveMetrics(ctx, dbTx, metrics); err != nil {
			return err
		}

		level, err := h.repo.GetLevelForUpdate(ctx, dbTx, cmd.AgentID)
		if err != nil {
			return err
		}

		oldRank = level.CurrentRank
		newRank := evaluateRank(metrics.AccumulatedVolume)
		finalRank = newRank

		if newRank != oldRank {
			promoted = true
			level.CurrentRank = newRank
			if err := h.repo.SaveLevel(ctx, dbTx, level); err != nil {
				return err
			}
		}

		now := time.Now()

		// Append VolumeAccumulated event
		volEvt := domain.VolumeAccumulated{
			TransactionID: cmd.TransactionID,
			AgentID:       cmd.AgentID,
			Amount:        cmd.Amount,
			TotalVolume:   metrics.AccumulatedVolume,
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

func evaluateRank(volume float64) domain.Rank {
	if volume >= 1000000 {
		return domain.RankDiamond
	} else if volume >= 100000 {
		return domain.RankPlatinum
	} else if volume >= 10000 {
		return domain.RankGold
	} else if volume >= 1000 {
		return domain.RankSilver
	}
	return domain.RankBronze
}
