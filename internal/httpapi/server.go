package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi/route"
	appmiddleware "github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module(
	"httpapi",
	fx.Provide(NewServer),
	fx.Invoke(RequireServer),
)

type Server struct {
	echo       *echo.Echo
	logger     logger.Factory
	cfg        *config.Config
	routes     []route.Registrar
	shutdowner fx.Shutdowner
}

type ServerParams struct {
	fx.In
	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Routes     []route.Registrar `group:"routes"`
	Config     *config.Config
	Logger     logger.Factory
}

func NewServer(p ServerParams) (*Server, error) {

	s := &Server{
		echo:       echo.New(),
		logger:     p.Logger,
		cfg:        p.Config,
		routes:     p.Routes,
		shutdowner: p.Shutdowner,
	}

	if err := s.setupHandlers(); err != nil {
		return nil, fmt.Errorf("failed to set up server: %w", err)
	}

	p.Lifecycle.Append(fx.Hook{OnStart: s.Start, OnStop: s.Stop})
	return s, nil
}

func RequireServer(*Server) {}

func (s *Server) setupHandlers() error {
	s.setupMiddlewares()
	s.setupRoutes()
	return nil
}

func (s *Server) setupMiddlewares() {
	s.echo.Use(
		s.corsMiddleware(),
		s.recoverMiddleware(),
		middleware.RequestID(),
		s.gzipMiddleware(),
		middleware.Secure(),
		middleware.BodyLimit("2M"),
	)

	s.echo.Use(appmiddleware.RequestLoggerMiddleware(s.logger))
	s.echo.Validator = &appmiddleware.CustomValidator{Validator: validator.New()}
	s.echo.HTTPErrorHandler = appmiddleware.ErrorHandler(s.logger)

	if s.cfg.Logger.Mode == "development" {
		s.echo.Use(appmiddleware.DebugMiddleware(*s.cfg, s.logger))
	}
}

func (s *Server) setupRoutes() {
	const extAuthzPrefix = "/internal/ext-authz/check"
	s.echo.Any(extAuthzPrefix+"/*", gatewayauth.ExtAuthzCheck(extAuthzPrefix, gatewayauth.DefaultRouteAuthorizer()))

	v1 := s.echo.Group("/api/v1")
	v1.GET("", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "Hello World")
	})
	s.setupHealthCheck(v1)
	route.MapRoutes(v1, s.routes)
}

func (s *Server) corsMiddleware() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXRequestID, echo.HeaderAuthorization, "traceparent", "tracestate", "baggage"},
	})
}

func (s *Server) recoverMiddleware() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize:         1 << 10, // 1 KB
		DisablePrintStack: true,
		DisableStackAll:   true,
	})
}

func (s *Server) gzipMiddleware() echo.MiddlewareFunc {
	return middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
		Skipper: func(c echo.Context) bool {
			return strings.Contains(c.Request().URL.Path, "swagger")
		},
	})
}

func (s *Server) setupHealthCheck(g *echo.Group) {
	health := g.Group("/health")
	health.GET("", func(c echo.Context) error {
		s.logger.Bg().Info("Health check ok!!!")
		return c.JSON(http.StatusOK, map[string]string{"status": "OK"})
	})
}

func (s *Server) Start(_ context.Context) error {
	address := ":" + strconv.Itoa(s.cfg.Server.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	s.echo.Listener = listener
	s.echo.Server.Addr = address
	s.logger.Bg().Info("Starting Echo server on address: %s", listener.Addr())
	go func() {
		if err := s.echo.StartServer(s.echo.Server); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Bg().Error("HTTP server stopped unexpectedly", zap.Error(err))
			if shutdownErr := s.shutdowner.Shutdown(fx.ExitCode(1)); shutdownErr != nil {
				s.logger.Bg().Error("Failed to request application shutdown", zap.Error(shutdownErr))
			}
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Bg().Info("Initiating graceful shutdown of the server")
	s.echo.Server.SetKeepAlivesEnabled(false)
	if err := s.echo.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shut down HTTP server: %w", err)
	}
	s.logger.Bg().Info("Server shutdown completed")
	return nil
}
