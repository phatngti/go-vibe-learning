package command

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type ConfigureCallbackHandler struct {
	uow          port.UnitOfWork
	repo         port.AgentRepository
	validator    port.CallbackEndpointValidator
	challenges   port.CallbackChallengeGenerator
	challengeTTL time.Duration
	now          func() time.Time
}

func NewConfigureCallbackHandler(
	uow port.UnitOfWork,
	repo port.AgentRepository,
	validator port.CallbackEndpointValidator,
	challenges port.CallbackChallengeGenerator,
	cfg *config.Config,
) *ConfigureCallbackHandler {
	return &ConfigureCallbackHandler{
		uow: uow, repo: repo, validator: validator, challenges: challenges,
		challengeTTL: cfg.Callback.ChallengeTTL, now: time.Now,
	}
}

func (h *ConfigureCallbackHandler) Handle(ctx context.Context, cmd *dto.ConfigureCallback) (*domain.CallbackConfiguration, error) {
	endpoint, err := h.validator.Validate(ctx, cmd.CallbackURL)
	if err != nil {
		return nil, err
	}
	challenge, err := h.challenges.Generate()
	if err != nil {
		return nil, err
	}
	proofURL, err := callbackChallengeProofURL(endpoint.URL, challenge.ID)
	if err != nil {
		return nil, err
	}
	issuedAt := h.now().UTC()
	expiresAt := issuedAt.Add(h.challengeTTL)
	var configured *domain.AgentConfig
	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		cfg, err := h.repo.GetConfig(txCtx, cmd.AgentID)
		if err != nil {
			return err
		}
		if cfg.TenantID != "" && cfg.TenantID != cmd.TenantID {
			return domain.ErrTenantOwnership
		}
		cfg.TenantID = cmd.TenantID
		cfg.CallbackURL = endpoint.URL
		cfg.CallbackHost = endpoint.Hostname
		cfg.CallbackStatus = domain.CallbackStatusPending
		cfg.CallbackVerifiedAt = nil
		cfg.CallbackChallengeID = challenge.ID
		cfg.CallbackChallengeDigest = challenge.Digest
		cfg.CallbackChallengeIssuedAt = &issuedAt
		cfg.CallbackChallengeExpiresAt = &expiresAt
		cfg.CallbackChallengeUsedAt = nil
		cfg.AllowedIPs = strings.Join(cmd.AllowedIPs, ",")
		if err := h.repo.SaveConfig(txCtx, cfg); err != nil {
			return err
		}

		audit := &domain.AuditLog{
			AgentID:  cmd.AgentID,
			TenantID: cmd.TenantID,
			Action:   "CONFIGURE_CALLBACK",
			Details:  fmt.Sprintf("Host: %s, Status: %s", cfg.CallbackHost, cfg.CallbackStatus),
		}

		if err := h.repo.CreateAuditLog(txCtx, audit); err != nil {
			return err
		}
		configured = cfg
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &domain.CallbackConfiguration{
		Config: configured,
		Challenge: domain.CallbackChallenge{
			AgentID: cmd.AgentID, TenantID: cmd.TenantID, ChallengeID: challenge.ID,
			Token: challenge.Token, ProofURL: proofURL, ExpiresAt: expiresAt,
			Status: domain.CallbackStatusPending,
		},
	}, nil
}

func callbackChallengeProofURL(callbackURL, challengeID string) (string, error) {
	parsed, err := url.Parse(callbackURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: cannot build proof URL", domain.ErrInvalidCallbackURL)
	}
	parsed.Path = "/.well-known/cqrs-callback-verification/" + url.PathEscape(challengeID)
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String(), nil
}
