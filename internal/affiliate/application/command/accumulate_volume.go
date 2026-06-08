package command

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/affiliate/application/dto"
	"github/master_go_cqrs/internal/affiliate/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type AccumulateVolumeHandler struct {
	db   *gorm.DB
	repo domain.AffiliateRepository
}

func NewAccumulateVolumeHandler(db *platformpostgres.DB, repo domain.AffiliateRepository) *AccumulateVolumeHandler {
	return &AccumulateVolumeHandler{db: db.DB, repo: repo}
}

func (h *AccumulateVolumeHandler) Handle(ctx context.Context, cmd *dto.AccumulateVolume) (*domain.AgentMetric, domain.Rank, bool, error) {
	var metrics *domain.AgentMetric
	var finalRank domain.Rank
	var promoted bool

	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		var err error
		metrics, err = h.repo.GetMetrics(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		metrics.AccumulatedVolume += cmd.Amount
		if err := h.repo.SaveMetrics(ctx, dbTx, metrics); err != nil {
			return err
		}

		level, err := h.repo.GetLevel(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		oldRank := level.CurrentRank
		newRank := evaluateRank(metrics.AccumulatedVolume)

		finalRank = newRank
		if newRank != oldRank {
			promoted = true
			level.CurrentRank = newRank
			if err := h.repo.SaveLevel(ctx, dbTx, level); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, "", false, fmt.Errorf("failed to accumulate volume: %w", err)
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
