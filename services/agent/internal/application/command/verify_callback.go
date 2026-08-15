package command

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type VerifyCallbackHandler struct {
	uow        port.UnitOfWork
	repo       port.AgentRepository
	validator  port.CallbackEndpointValidator
	challenges port.CallbackChallengeGenerator
	prover     port.CallbackOwnershipProver
	now        func() time.Time
}

func NewVerifyCallbackHandler(
	uow port.UnitOfWork,
	repo port.AgentRepository,
	validator port.CallbackEndpointValidator,
	challenges port.CallbackChallengeGenerator,
	prover port.CallbackOwnershipProver,
) *VerifyCallbackHandler {
	return &VerifyCallbackHandler{
		uow: uow, repo: repo, validator: validator, challenges: challenges,
		prover: prover, now: time.Now,
	}
}

func (h *VerifyCallbackHandler) Handle(ctx context.Context, cmd *dto.VerifyCallback) (*domain.AgentConfig, error) {
	cfg, err := h.repo.GetConfig(ctx, strings.TrimSpace(cmd.AgentID))
	if err != nil {
		return nil, err
	}
	now := h.now().UTC()
	if err := h.validateChallenge(cfg, cmd, now); err != nil {
		return nil, err
	}
	endpoint, err := h.validator.Validate(ctx, cfg.CallbackURL)
	if err != nil {
		return nil, err
	}
	proofURL, err := callbackChallengeProofURL(endpoint.URL, cfg.CallbackChallengeID)
	if err != nil {
		return nil, err
	}
	// Provider I/O deliberately occurs before the short commit transaction.
	if err := h.prover.Verify(ctx, proofURL, cmd.ChallengeToken); err != nil {
		return nil, err
	}

	var verified *domain.AgentConfig
	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		fresh, err := h.repo.GetConfig(txCtx, strings.TrimSpace(cmd.AgentID))
		if err != nil {
			return err
		}
		commitTime := h.now().UTC()
		if err := h.validateChallenge(fresh, cmd, commitTime); err != nil {
			return err
		}
		fresh.CallbackStatus = domain.CallbackStatusVerified
		fresh.CallbackVerifiedAt = &commitTime
		fresh.CallbackChallengeUsedAt = &commitTime
		if err := h.repo.SaveConfig(txCtx, fresh); err != nil {
			return err
		}
		if err := h.repo.CreateAuditLog(txCtx, &domain.AuditLog{
			AgentID: fresh.AgentID, TenantID: fresh.TenantID,
			Action:  "VERIFY_CALLBACK_OWNERSHIP",
			Details: fmt.Sprintf("Host: %s, Challenge: %s", fresh.CallbackHost, fresh.CallbackChallengeID),
		}); err != nil {
			return err
		}
		verified = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return verified, nil
}

func (h *VerifyCallbackHandler) validateChallenge(cfg *domain.AgentConfig, cmd *dto.VerifyCallback, now time.Time) error {
	if cfg == nil || cfg.CallbackChallengeID == "" || cfg.CallbackChallengeDigest == "" || cfg.CallbackChallengeExpiresAt == nil {
		return domain.ErrCallbackChallengeMissing
	}
	if cfg.TenantID == "" || cfg.TenantID != strings.TrimSpace(cmd.TenantID) || cfg.AgentID != strings.TrimSpace(cmd.AgentID) {
		return domain.ErrTenantOwnership
	}
	if cfg.CallbackChallengeUsedAt != nil || cfg.CallbackStatus == domain.CallbackStatusVerified {
		return domain.ErrCallbackChallengeReplayed
	}
	if cfg.CallbackChallengeID != strings.TrimSpace(cmd.ChallengeID) {
		return domain.ErrCallbackChallengeInvalid
	}
	if !now.Before(cfg.CallbackChallengeExpiresAt.UTC()) {
		return domain.ErrCallbackChallengeExpired
	}
	digest := h.challenges.Digest(strings.TrimSpace(cmd.ChallengeToken))
	if subtle.ConstantTimeCompare([]byte(digest), []byte(cfg.CallbackChallengeDigest)) != 1 {
		return domain.ErrCallbackChallengeInvalid
	}
	return nil
}
