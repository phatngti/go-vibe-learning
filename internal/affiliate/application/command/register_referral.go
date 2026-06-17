package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/affiliate/application/dto"
	"github/master_go_cqrs/internal/affiliate/domain"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type RegisterReferralHandler struct {
	db         *gorm.DB
	repo       domain.AffiliateRepository
	eventStore eventstore.Repository
}

func NewRegisterReferralHandler(db *platformpostgres.DB, repo domain.AffiliateRepository, es eventstore.Repository) *RegisterReferralHandler {
	return &RegisterReferralHandler{db: db.DB, repo: repo, eventStore: es}
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

		// Append ReferralRegistered domain event
		evt := domain.ReferralRegistered{
			ReferrerID: cmd.ReferrerID,
			RefereeID:  cmd.RefereeID,
			OccurredAt: time.Now(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("register_referral: marshal event: %w", err)
		}
		version, err := h.eventStore.NextVersion(ctx, dbTx, "Referral", cmd.RefereeID)
		if err != nil {
			return err
		}
		return h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
			AggregateType: "Referral",
			AggregateID:   cmd.RefereeID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to register referral: %w", err)
	}
	return nil
}
