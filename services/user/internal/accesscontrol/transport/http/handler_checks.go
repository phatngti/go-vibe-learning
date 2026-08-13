package http

import (
	"net/http"

	"github/master_go_cqrs/internal/platform/httpapi/response"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/accesscontrol/application/dto"

	"github.com/labstack/echo/v4"
)

// checkAuthnHandler godoc
//
//	@Summary		Check authentication
//	@Description	Return authenticated user details for a valid bearer token.
//	@Tags			access-control
//	@Produce		json
//	@Success		200	{object}	dto.AuthnCheckResponseDTO
//	@Failure		401	{object}	response.ErrorResponse
//	@Security		BearerAuth
//	@Router			/author/authn/check [get]
func (h *Handler) checkAuthnHandler(c echo.Context) error {
	payload := h.checkAuthn.Handle(c.Request().Context(), toDomainUser(c.Get("user").(gatewayauth.User)))
	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.AuthnCheckSuccess, payload))
}

// checkRBACHandler godoc
//
//	@Summary		Check RBAC
//	@Description	Check whether the authenticated user has all requested scopes on a resource.
//	@Tags			access-control
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CheckRBACRequestDTO	true	"RBAC check payload"
//	@Success		200		{object}	dto.RBACCheckResponseDTO
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		401		{object}	response.ErrorResponse
//	@Failure		403		{object}	dto.RBACCheckResponseDTO
//	@Security		BearerAuth
//	@Router			/author/rbac/check [post]
func (h *Handler) checkRBACHandler(c echo.Context) error {
	request := c.Get("dto").(*dto.CheckRBACRequestDTO)
	payload := h.checkRBAC.Handle(c.Request().Context(), toDomainUser(c.Get("user").(gatewayauth.User)), dto.CheckRBACRequest{
		Resource: request.Resource,
		Scopes:   request.Scopes,
	})

	if !payload.Allowed {
		return c.JSON(http.StatusForbidden, response.Wrap(dto.MessageCode, dto.RBACCheckFailed, payload))
	}
	return c.JSON(http.StatusOK, response.Wrap(dto.MessageCode, dto.RBACCheckSuccess, payload))
}
