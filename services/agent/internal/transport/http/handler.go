package http

import (
	"errors"
	"net/http"
	"time"

	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/agent/internal/application/command"
	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/domain"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger            logger.Factory
	configureCallback *command.ConfigureCallbackHandler
	verifyCallback    *command.VerifyCallbackHandler
	replayCallback    *command.ReplayCallbackHandler
	generateAPIKey    *command.GenerateAPIKeyHandler
	authenticate      gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger            logger.Factory
	ConfigureCallback *command.ConfigureCallbackHandler
	VerifyCallback    *command.VerifyCallbackHandler
	ReplayCallback    *command.ReplayCallbackHandler
	GenerateAPIKey    *command.GenerateAPIKeyHandler
	TokenAuth         gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:            params.Logger,
		configureCallback: params.ConfigureCallback,
		verifyCallback:    params.VerifyCallback,
		replayCallback:    params.ReplayCallback,
		generateAPIKey:    params.GenerateAPIKey,
		authenticate:      params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return ""
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	agent := g.Group("/agent")
	agent.POST("/config/callback", h.ConfigureCallback,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.ConfigureCallback), dto.MessageCode),
	)
	agent.POST("/config/callback/verify", h.VerifyCallback,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.VerifyCallback), dto.MessageCode),
	)
	agent.POST("/config/key/generate", h.GenerateApiKey,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.GenerateAPIKey), dto.MessageCode),
	)
	agent.POST("/callbacks/:event_id/replay", h.ReplayCallback,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.ReplayCallback), dto.MessageCode),
	)
}

// ConfigureCallback godoc
//
//	@Summary		Configure agent callback
//	@Description	Update callback URL and allowed IPs for an agent.
//	@Tags			agent
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.ConfigureCallback	true	"Callback configuration payload"
//	@Success		200		{object}	dto.AgentConfigResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/agent/config/callback [post]
func (h *Handler) ConfigureCallback(c echo.Context) error {
	req := c.Get("dto").(*dto.ConfigureCallback)
	ownership, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), req.AgentID)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	req.AgentID = ownership.AgentID
	req.TenantID = ownership.TenantID

	cfg, err := h.configureCallback.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		if errors.Is(err, domain.ErrInvalidCallbackURL) || errors.Is(err, domain.ErrCallbackNotVerified) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if errors.Is(err, domain.ErrTenantOwnership) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	configured := cfg.Config
	return c.JSON(http.StatusAccepted, dto.AgentConfigResponse{
		AgentID: configured.AgentID, CallbackURL: configured.CallbackURL,
		CallbackStatus: configured.CallbackStatus, AllowedIPs: configured.AllowedIPs,
		UpdatedAt:   configured.UpdatedAt.Format(time.RFC3339),
		ChallengeID: cfg.Challenge.ChallengeID, ChallengeToken: cfg.Challenge.Token,
		ChallengeProofURL:  cfg.Challenge.ProofURL,
		ChallengeExpiresAt: cfg.Challenge.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) VerifyCallback(c echo.Context) error {
	req := c.Get("dto").(*dto.VerifyCallback)
	ownership, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), req.AgentID)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	req.AgentID = ownership.AgentID
	req.TenantID = ownership.TenantID

	cfg, err := h.verifyCallback.Handle(c.Request().Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTenantOwnership):
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		case errors.Is(err, domain.ErrCallbackChallengeReplayed):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, domain.ErrCallbackChallengeMissing),
			errors.Is(err, domain.ErrCallbackChallengeExpired),
			errors.Is(err, domain.ErrCallbackChallengeInvalid):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "callback verification failed"})
		}
	}
	verifiedAt := ""
	if cfg.CallbackVerifiedAt != nil {
		verifiedAt = cfg.CallbackVerifiedAt.UTC().Format(time.RFC3339)
	}
	return c.JSON(http.StatusOK, dto.VerifyCallbackResponse{
		AgentID: cfg.AgentID, CallbackURL: cfg.CallbackURL,
		CallbackStatus: cfg.CallbackStatus, CallbackVerified: verifiedAt,
	})
}

func (h *Handler) ReplayCallback(c echo.Context) error {
	request := c.Get("dto").(*dto.ReplayCallback)
	principal, err := gatewayauth.RequireUser(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}
	request.EventID = c.Param("event_id")
	request.TenantID = principal.TenantID
	request.OperatorID = principal.ID
	request.PlatformOperator = principal.HasRole("platform_operator")
	delivery, err := h.replayCallback.Handle(c.Request().Context(), request)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCallbackReplayForbidden), errors.Is(err, domain.ErrTenantOwnership):
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		case errors.Is(err, domain.ErrCallbackDeliveryNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		case errors.Is(err, domain.ErrCallbackDeliveryNotDead):
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "callback replay failed"})
		}
	}
	return c.JSON(http.StatusAccepted, dto.ReplayCallbackResponse{
		EventID: delivery.EventID, Status: delivery.Status,
		OutcomeGeneration: delivery.OutcomeGeneration, ReplayCount: delivery.ReplayCount,
	})
}

// GenerateApiKey godoc
//
//	@Summary		Generate agent API key
//	@Description	Generate and persist a new API key for an agent.
//	@Tags			agent
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.GenerateAPIKey	true	"API key generation payload"
//	@Success		200		{object}	dto.GenerateAPIKeyResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/agent/config/key/generate [post]
func (h *Handler) GenerateApiKey(c echo.Context) error {
	req := c.Get("dto").(*dto.GenerateAPIKey)
	ownership, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), req.AgentID)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	req.AgentID = ownership.AgentID
	req.TenantID = ownership.TenantID

	cfg, err := h.generateAPIKey.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.GenerateAPIKeyResponse{
		AgentID: cfg.AgentID,
		APIKey:  cfg.ApiKey,
	})
}
