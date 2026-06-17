package http

import (
	"net/http"
	"time"

	"github/master_go_cqrs/internal/agent/application/command"
	"github/master_go_cqrs/internal/agent/application/dto"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger            logger.Factory
	configureCallback *command.ConfigureCallbackHandler
	generateAPIKey    *command.GenerateAPIKeyHandler
	authenticate      gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger            logger.Factory
	ConfigureCallback *command.ConfigureCallbackHandler
	GenerateAPIKey    *command.GenerateAPIKeyHandler
	TokenAuth         gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:            params.Logger,
		configureCallback: params.ConfigureCallback,
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
	agent.POST("/config/key/generate", h.GenerateApiKey,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.GenerateAPIKey), dto.MessageCode),
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

	cfg, err := h.configureCallback.Handle(c.Request().Context(), req)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.AgentConfigResponse{
		AgentID:     cfg.AgentID,
		CallbackURL: cfg.CallbackURL,
		APIKey:      cfg.ApiKey,
		AllowedIPs:  cfg.AllowedIPs,
		UpdatedAt:   cfg.UpdatedAt.Format(time.RFC3339),
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
