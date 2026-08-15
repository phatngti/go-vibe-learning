package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	appmiddleware "github/master_go_cqrs/internal/platform/middleware"
	"github/master_go_cqrs/internal/platform/observability/logger"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"

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
	echo          *echo.Echo
	logger        logger.Factory
	serverCfg     *config.Server
	loggerCfg     *config.Logger
	routes        []route.Registrar
	v2Routes      []route.Registrar
	shutdowner    fx.Shutdowner
	listen        func(network, address string) (net.Listener, error)
	readiness     interface{ PingContext(context.Context) error }
	authenticator gatewayauth.PrincipalAuthenticator
}

type ServerParams struct {
	fx.In
	Lifecycle     fx.Lifecycle
	Shutdowner    fx.Shutdowner
	Routes        []route.Registrar `group:"routes"`
	V2Routes      []route.Registrar `group:"routes_v2"`
	Server        *config.Server
	LoggerConfig  *config.Logger
	Logger        logger.Factory
	Database      *platformpostgres.DB               `optional:"true"`
	Authenticator gatewayauth.PrincipalAuthenticator `optional:"true"`
}

func NewServer(p ServerParams) (*Server, error) {

	authenticator := p.Authenticator
	if authenticator == nil {
		authenticator, _ = gatewayauth.NewJWTAuthenticator(&gatewayauth.VerificationConfig{})
	}
	s := &Server{
		echo:          echo.New(),
		logger:        p.Logger,
		serverCfg:     p.Server,
		loggerCfg:     p.LoggerConfig,
		routes:        p.Routes,
		v2Routes:      p.V2Routes,
		shutdowner:    p.Shutdowner,
		listen:        net.Listen,
		authenticator: authenticator,
	}
	if p.Database != nil && p.Database.DB != nil {
		sqlDB, err := p.Database.DB.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to configure readiness probe: %w", err)
		}
		s.readiness = sqlDB
	}
	s.echo.Server.ReadHeaderTimeout = positiveDuration(p.Server.ReadHeaderTimeout, 5*time.Second)
	s.echo.Server.ReadTimeout = positiveDuration(p.Server.ReadTimeout, 15*time.Second)
	s.echo.Server.WriteTimeout = positiveDuration(p.Server.WriteTimeout, 30*time.Second)
	s.echo.Server.IdleTimeout = positiveDuration(p.Server.IdleTimeout, 60*time.Second)

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

	if s.loggerCfg.Mode == "development" {
		s.echo.Use(appmiddleware.DebugMiddleware(*s.loggerCfg, s.logger))
	}
}

func (s *Server) setupRoutes() {
	const extAuthzPrefix = "/internal/ext-authz/check"
	s.echo.Any(extAuthzPrefix+"/*", gatewayauth.ExtAuthzCheckWithAuthenticator(extAuthzPrefix, gatewayauth.DefaultRouteAuthorizer(), s.authenticator))

	v1 := s.echo.Group("/api/v1")
	v1.GET("", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "Hello World")
	})
	s.setupHealthCheck(v1)
	route.MapRoutes(v1, s.routes)

	v2 := s.echo.Group("/api/v2")
	route.MapRoutes(v2, s.v2Routes)
}

func (s *Server) corsMiddleware() echo.MiddlewareFunc {
	allowedOrigins := s.serverCfg.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowedOrigins,
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXRequestID, echo.HeaderAuthorization, "Idempotency-Key", "traceparent", "tracestate", "baggage"},
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
	health.GET("/live", s.liveness)
	health.GET("/ready", s.readinessCheck)
	// Backward-compatible alias. Kubernetes manifests should use /ready.
	health.GET("", s.readinessCheck)
}

func (s *Server) liveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "OK"})
}

func (s *Server) readinessCheck(c echo.Context) error {
	if s.readiness == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "READY"})
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := s.readiness.PingContext(ctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "NOT_READY"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "READY"})
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func (s *Server) Start(_ context.Context) error {
	address := ":" + strconv.Itoa(s.serverCfg.Port)
	listener, err := s.listen("tcp", address)
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
