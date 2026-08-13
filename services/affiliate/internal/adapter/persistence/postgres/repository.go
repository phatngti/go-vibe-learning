package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/affiliate/internal/application/port"
	"github/master_go_cqrs/services/affiliate/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type affiliateRepository struct{ db *gorm.DB }

func NewAffiliateRepository(db *platformpostgres.DB) port.AffiliateRepository {
	return &affiliateRepository{db: db.DB}
}

func (r *affiliateRepository) SaveReferral(ctx context.Context, ref *domain.Referral) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := referralToRow(ref)
	if err := activeDB.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("failed to save referral: %w", err)
	}
	return nil
}

func (r *affiliateRepository) GetDownline(ctx context.Context, agentID string) ([]string, error) {
	var refereeIDs []string
	err := r.db.WithContext(ctx).
		Model(&ReferralRow{}).
		Where("referrer_id = ?", agentID).
		Pluck("referee_id", &refereeIDs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query downline referees: %w", err)
	}
	return refereeIDs, nil
}

func (r *affiliateRepository) GetMetrics(ctx context.Context, agentID string) (*domain.AgentMetric, error) {
	return r.getMetrics(ctx, r.db, agentID, false)
}

func (r *affiliateRepository) GetMetricsForUpdate(ctx context.Context, agentID string) (*domain.AgentMetric, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	return r.getMetrics(ctx, activeDB, agentID, true)
}

func (r *affiliateRepository) getMetrics(ctx context.Context, db *gorm.DB, agentID string, lock bool) (*domain.AgentMetric, error) {
	var row AgentMetricRow
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&row, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentMetric{AgentID: agentID}, nil
		}
		return nil, fmt.Errorf("failed to load agent metrics: %w", err)
	}
	return metricToDomain(&row), nil
}

func (r *affiliateRepository) SaveMetrics(ctx context.Context, metric *domain.AgentMetric) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	metric.UpdatedAt = time.Now()
	row := metricToRow(metric)
	if err := activeDB.WithContext(ctx).Save(row).Error; err != nil {
		return fmt.Errorf("failed to save agent metrics: %w", err)
	}
	return nil
}

func (r *affiliateRepository) GetLevel(ctx context.Context, agentID string) (*domain.AffiliateLevel, error) {
	return r.getLevel(ctx, r.db, agentID, false)
}

func (r *affiliateRepository) GetLevelForUpdate(ctx context.Context, agentID string) (*domain.AffiliateLevel, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	return r.getLevel(ctx, activeDB, agentID, true)
}

func (r *affiliateRepository) getLevel(ctx context.Context, db *gorm.DB, agentID string, lock bool) (*domain.AffiliateLevel, error) {
	var row AffiliateLevelRow
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&row, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AffiliateLevel{AgentID: agentID, CurrentRank: domain.RankBronze}, nil
		}
		return nil, fmt.Errorf("failed to load affiliate level: %w", err)
	}
	return levelToDomain(&row), nil
}

func (r *affiliateRepository) SaveLevel(ctx context.Context, lvl *domain.AffiliateLevel) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	lvl.PromotedAt = time.Now()
	row := levelToRow(lvl)
	if err := activeDB.WithContext(ctx).Save(row).Error; err != nil {
		return fmt.Errorf("failed to save affiliate level: %w", err)
	}
	return nil
}
