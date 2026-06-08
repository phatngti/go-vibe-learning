package http

import (
	"github/master_go_cqrs/internal/accesscontrol/application/dto"
	"github/master_go_cqrs/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/authn/check",
		h.checkAuthnHandler,
		h.authenticate.Authorize(),
	)

	g.POST("/rbac/check",
		h.checkRBACHandler,
		h.authenticate.Authorize(),
		middleware.ValidateHandler(new(dto.CheckRBACRequestDTO), dto.MessageCode),
	)
}
