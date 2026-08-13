package http

import (
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/accesscontrol/application/query"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type Handler struct {
	logger       logger.Factory
	checkAuthn   *query.CheckAuthnHandler
	checkRBAC    *query.CheckRBACHandler
	authenticate gatewayauth.Middleware
}

type HandlerParams struct {
	fx.In
	Logger     logger.Factory
	CheckAuthn *query.CheckAuthnHandler
	CheckRBAC  *query.CheckRBACHandler
	TokenAuth  gatewayauth.Middleware
}

func NewHandler(params HandlerParams) route.Registrar {
	return &Handler{
		logger:       params.Logger,
		checkAuthn:   params.CheckAuthn,
		checkRBAC:    params.CheckRBAC,
		authenticate: params.TokenAuth,
	}
}

func (h *Handler) Pattern(prefix *echo.Group) string {
	return "/author"
}
