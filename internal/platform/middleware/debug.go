package middleware

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/labstack/echo/v4"
)

func DebugMiddleware(config config.Logger, logger logger.Factory) echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Mode == "development" {
				dump, err := httputil.DumpRequest(c.Request(), true)
				if err != nil {
					return c.NoContent(http.StatusInternalServerError)
				}
				logger.Bg().Info(fmt.Sprintf("\nRequest dump begin :--------------\n\n%s\n\nRequest dump end :--------------", dump))
			}

			err := next(c)
			return err

		}
	}
}
