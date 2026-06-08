package middleware

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"github.com/labstack/echo/v4"
)

func DebugMiddleware(config config.Config, logger logger.Factory) echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Logger.Mode == "development" {
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
