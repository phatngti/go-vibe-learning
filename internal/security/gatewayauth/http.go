package gatewayauth

import (
	"net/http"

	"github/master_go_cqrs/internal/httpapi/response"

	"github.com/labstack/echo/v4"
)

const (
	GatewayAuthFailed  = "gateway_auth_failed"
	GatewayAuthzFailed = "gateway_authorize_failed"
)

var MessageCode = map[string]string{
	GatewayAuthFailed:  "Gateway authentication failed",
	GatewayAuthzFailed: "Gateway authorization failed",
}

type Middleware interface {
	Authorize() echo.MiddlewareFunc
	AuthorizeWithResource(resource string, scopes []string) echo.MiddlewareFunc
}

type middleware struct{}

func NewMiddleware() Middleware {
	return middleware{}
}

func (m middleware) Authorize() echo.MiddlewareFunc {
	return m.authorize("", nil)
}

func (m middleware) AuthorizeWithResource(resource string, scopes []string) echo.MiddlewareFunc {
	return m.authorize(resource, scopes)
}

func (m middleware) authorize(resource string, scopes []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := DecodeUserPayload(c.Request().Header.Get(PayloadHeader))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, response.WrapError(MessageCode, GatewayAuthFailed, nil))
			}
			if resource != "" && !HasResourceScopes(user.ResourceAccess, resource, scopes) {
				return echo.NewHTTPError(http.StatusForbidden, response.WrapError(MessageCode, GatewayAuthzFailed, nil))
			}

			c.Set("user", user)
			c.Set("gateway_user", user)
			c.SetRequest(c.Request().WithContext(WithUser(c.Request().Context(), user)))
			return next(c)
		}
	}
}

func ExtAuthzCheck(prefix string, authorizer RouteAuthorizer) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := DecodeUserPayload(c.Request().Header.Get(PayloadHeader))
		if err != nil {
			return c.NoContent(http.StatusUnauthorized)
		}

		path := c.Request().URL.Path
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			path = path[len(prefix):]
		}
		if path == "" {
			path = "/"
		}

		if !authorizer.Authorize(c.Request().Method, path, user) {
			return c.NoContent(http.StatusForbidden)
		}
		return c.NoContent(http.StatusOK)
	}
}
