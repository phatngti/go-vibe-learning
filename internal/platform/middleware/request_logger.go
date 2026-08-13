package middleware

import (
	"time"

	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/labstack/echo/v4"
)

func RequestLoggerMiddleware(logger logger.Factory) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			start := time.Now()
			err := next(ctx)
			req := ctx.Request()
			res := ctx.Response()
			status := res.Status
			size := res.Size
			s := time.Since(start).String()
			logger.Bg().Info("HTTP method=%s, uri=%s, status=%d, size=%d, time=%s", req.Method, req.URL.String(), status, size, s)

			return err
		}
	}
}
