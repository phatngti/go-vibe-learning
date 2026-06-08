package command

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/affiliate/application/dto"
	"github/master_go_cqrs/internal/affiliate/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type RegisterReferralHandler struct {
	db   *gorm.DB
	repo domain.AffiliateRepository
}

func NewRegisterReferralHandler(db *platformpostgres.DB, repo domain.AffiliateRepository) *RegisterReferralHandler {
	return &RegisterReferralHandler{db: db.DB, repo: repo}
}

func (h *RegisterReferralHandler) Handle(ctx context.Context, cmd *dto.RegisterReferral) error {
	ref := &domain.Referral{
		ReferrerID: cmd.ReferrerID,
		RefereeID:  cmd.RefereeID,
	}

	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.SaveReferral(ctx, dbTx, ref); err != nil {
			return err
		}

		metrics, err := h.repo.GetMetrics(ctx, cmd.ReferrerID)
		if err != nil {
			return err
		}

		metrics.ReferralCount++
		if err := h.repo.SaveMetrics(ctx, dbTx, metrics); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to register referral: %w", err)
	}
	return nil
}
