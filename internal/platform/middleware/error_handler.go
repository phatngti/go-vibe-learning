package middleware

import (
	"fmt"

	"github/master_go_cqrs/internal/platform/httpapi/response"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func ErrorHandler(logger logger.Factory) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		// If no error, do nothing and let the handler proceed
		ctx, span := otel.Tracer("error_handler").Start(c.Request().Context(), "ErrorHandler")
		defer span.End()

		if err == nil {
			return
		}

		// If response is already committed, avoid writing again
		if c.Response().Committed {
			return
		}

		// Handle Echo HTTP errors
		if he, ok := err.(*echo.HTTPError); ok {
			// Write JSON response with the error details
			message, ok := (he.Message).(response.ErrorResponse)
			if !ok {
				message = response.ErrorResponse{
					Code:    fmt.Sprintf("%d", he.Code),
					Message: fmt.Sprintf("%v", he.Message),
				}
			}

			logger.For(ctx).WithShowTerminal(true).Error("http_code=%d, code=%s, message=%s",
				he.Code,
				message.Code,
				message.Message,
			).WithFields(
				zap.Int("http_code", he.Code),
				zap.String("code", message.Code),
				zap.String("message", message.Message),
			)
			if err := c.JSON(he.Code, message); err != nil {
				// logger.For(ctx).WithShowTerminal(true).Error("Failed to write error response:" + fmt.Sprintf("%v", err))
			}
			return
		}

		// Fallback for unexpected errors
		message := err.Error()
		// logger.For(ctx).WithShowTerminal(true).Error("http_code=%d, code=%d, message=%s",
		// 	echo.ErrInternalServerError.Code,
		// 	echo.ErrInternalServerError.Code,
		// 	message,
		// ).WithFields(
		// 	zap.Int("http_code", echo.ErrInternalServerError.Code),
		// 	zap.String("message", message),
		// )
		if err := c.JSON(echo.ErrInternalServerError.Code,
			response.ErrorResponse{
				Code:    "internal_server_error",
				Message: message,
				Errors:  nil,
			},
		); err != nil {
			// logger.For(ctx).WithShowTerminal(true).Error("Failed to write fallback error response:" + fmt.Sprintf("%v", err)).WithFields(zap.Error(err))
		}

	}
}
