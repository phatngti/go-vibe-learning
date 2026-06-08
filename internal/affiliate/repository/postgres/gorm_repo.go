package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/affiliate/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type affiliateRepository struct {
	db *gorm.DB
}

func NewAffiliateRepository(db *platformpostgres.DB) domain.AffiliateRepository {
	return &affiliateRepository{db: db.DB}
}

func (r *affiliateRepository) SaveReferral(ctx context.Context, db *gorm.DB, ref *domain.Referral) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	if err := activeDB.WithContext(ctx).Create(ref).Error; err != nil {
		return fmt.Errorf("failed to save referral: %w", err)
	}
	return nil
}

func (r *affiliateRepository) GetDownline(ctx context.Context, agentID string) ([]string, error) {
	var refereeIDs []string
	err := r.db.WithContext(ctx).
		Model(&domain.Referral{}).
		Where("referrer_id = ?", agentID).
		Pluck("referee_id", &refereeIDs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query downline referees: %w", err)
	}
	return refereeIDs, nil
}

func (r *affiliateRepository) GetMetrics(ctx context.Context, agentID string) (*domain.AgentMetric, error) {
	var metric domain.AgentMetric
	err := r.db.WithContext(ctx).First(&metric, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AgentMetric{AgentID: agentID}, nil
		}
		return nil, fmt.Errorf("failed to load agent metrics: %w", err)
	}
	return &metric, nil
}

func (r *affiliateRepository) SaveMetrics(ctx context.Context, db *gorm.DB, metric *domain.AgentMetric) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	metric.UpdatedAt = time.Now()
	err := activeDB.WithContext(ctx).
		Save(metric).Error
	if err != nil {
		return fmt.Errorf("failed to save agent metrics: %w", err)
	}
	return nil
}

func (r *affiliateRepository) GetLevel(ctx context.Context, agentID string) (*domain.AffiliateLevel, error) {
	var lvl domain.AffiliateLevel
	err := r.db.WithContext(ctx).First(&lvl, "agent_id = ?", agentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.AffiliateLevel{AgentID: agentID, CurrentRank: domain.RankBronze}, nil
		}
		return nil, fmt.Errorf("failed to load affiliate level: %w", err)
	}
	return &lvl, nil
}

func (r *affiliateRepository) SaveLevel(ctx context.Context, db *gorm.DB, lvl *domain.AffiliateLevel) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	lvl.PromotedAt = time.Now()
	err := activeDB.WithContext(ctx).
		Save(lvl).Error
	if err != nil {
		return fmt.Errorf("failed to save affiliate level: %w", err)
	}
	return nil
}
