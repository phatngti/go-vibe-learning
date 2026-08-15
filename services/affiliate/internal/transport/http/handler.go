package http

import (
	"net/http"

	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/affiliate/internal/application/command"
	"github/master_go_cqrs/services/affiliate/internal/application/dto"
	"github/master_go_cqrs/services/affiliate/internal/application/query"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	logger           logger.Factory
	registerReferral *command.RegisterReferralHandler
	getRank          *query.GetRankHandler
	getDownline      *query.GetDownlineHandler
	authenticate     gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger           logger.Factory
	RegisterReferral *command.RegisterReferralHandler
	GetRank          *query.GetRankHandler
	GetDownline      *query.GetDownlineHandler
	TokenAuth        gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:           params.Logger,
		registerReferral: params.RegisterReferral,
		getRank:          params.GetRank,
		getDownline:      params.GetDownline,
		authenticate:     params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return ""
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	affiliate := g.Group("/affiliate")
	affiliate.POST("/referral/link", h.LinkReferral,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.RegisterReferral), dto.MessageCode),
	)
	affiliate.GET("/rank/:id", h.GetRank,
		h.authenticate.Authorize(),
	)
	affiliate.GET("/downline/:id", h.GetDownline,
		h.authenticate.Authorize(),
	)
}

// LinkReferral godoc
//
//	@Summary		Link affiliate referral
//	@Description	Create a referral link between a referrer and referee.
//	@Tags			affiliate
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.RegisterReferral	true	"Referral payload"
//	@Success		200		{object}	dto.RegisterReferralResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/affiliate/referral/link [post]
func (h *Handler) LinkReferral(c echo.Context) error {
	req := c.Get("dto").(*dto.RegisterReferral)
	owner, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), req.ReferrerID)
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	req.ReferrerID = owner.AgentID

	if err := h.registerReferral.Handle(c.Request().Context(), req); err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.RegisterReferralResponse{Status: "linked"})
}

// GetRank godoc
//
//	@Summary		Get affiliate rank
//	@Description	Return rank and volume metrics for an agent.
//	@Tags			affiliate
//	@Produce		json
//	@Param			id	path		string	true	"Agent ID"
//	@Success		200	{object}	dto.RankResponse
//	@Failure		404	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/affiliate/rank/{id} [get]
func (h *Handler) GetRank(c echo.Context) error {
	owner, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	id := owner.AgentID
	level, metrics, err := h.getRank.Handle(c.Request().Context(), id)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Agent metrics/rank not found"})
	}

	return c.JSON(http.StatusOK, dto.RankResponse{
		AgentID:     level.AgentID,
		CurrentRank: string(level.CurrentRank),
		TotalMoney:  metrics.AccumulatedVolumeMoney,
		TotalVolume: metrics.AccumulatedVolume,
		Referrals:   metrics.ReferralCount,
	})
}

// GetDownline godoc
//
//	@Summary		Get affiliate downline
//	@Description	Return all direct referee IDs for an agent.
//	@Tags			affiliate
//	@Produce		json
//	@Param			id	path		string	true	"Agent ID"
//	@Success		200	{object}	dto.DownlineResponse
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/affiliate/downline/{id} [get]
func (h *Handler) GetDownline(c echo.Context) error {
	owner, err := gatewayauth.RequireAgentOwnership(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
	}
	id := owner.AgentID
	referees, err := h.getDownline.Handle(c.Request().Context(), id)
	if err != nil {
		h.logger.For(c.Request().Context()).Error("%v", err.Error()).WithFields(zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.DownlineResponse{
		AgentID:        id,
		RefereeIDs:     referees,
		TotalReferrals: len(referees),
	})
}
