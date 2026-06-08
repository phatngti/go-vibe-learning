package httpapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	accesscontrolquery "github/master_go_cqrs/internal/accesscontrol/application/query"
	accesscontrolhttp "github/master_go_cqrs/internal/accesscontrol/delivery/http"
	apphttp "github/master_go_cqrs/internal/app/delivery/http"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx/fxtest"
)

type allowAll struct{}

func (allowAll) Authorize() echo.MiddlewareFunc {
	return passthrough
}

func (allowAll) AuthorizeWithResource(string, []string) echo.MiddlewareFunc {
	return passthrough
}

func passthrough(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("user", gatewayauth.User{ID: "test-user"})
		return next(c)
	}
}

func TestServerRoutes(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	registrars := []route.Registrar{
		apphttp.NewHandler(apphttp.HandlerParams{TokenAuth: allowAll{}}),
		accesscontrolhttp.NewHandler(accesscontrolhttp.HandlerParams{
			CheckAuthn: accesscontrolquery.NewCheckAuthnHandler(),
			CheckRBAC:  accesscontrolquery.NewCheckRBACHandler(),
			TokenAuth:  allowAll{},
		}),
	}

	server, err := NewServer(ServerParams{
		Lifecycle: lifecycle,
		Config:    new(config.Config),
		Routes:    registrars,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	routes := make(map[string]struct{})
	for _, registered := range server.echo.Routes() {
		routes[registered.Method+" "+registered.Path] = struct{}{}
	}

	expected := []string{
		"GET /api/v1",
		"GET /api/v1/health",
		"GET /api/v1/auth/verify-token",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/refresh",
		"GET /api/v1/auth/info",
		"GET /api/v1/author/authn/check",
		"POST /api/v1/author/rbac/check",
		"GET /internal/ext-authz/check/*",
	}

	for _, expectedRoute := range expected {
		if _, ok := routes[expectedRoute]; !ok {
			t.Errorf("missing route %q", expectedRoute)
		}
	}
}

func TestServerLifecycle(t *testing.T) {
	cfg := new(config.Config)
	cfg.Server.Port = 0
	lifecycle := fxtest.NewLifecycle(t)
	server, err := NewServer(ServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Logger:    newTestLoggerFactory(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	lifecycle.RequireStart()
	if server.echo.ListenerAddr() == nil {
		t.Fatal("ListenerAddr() = nil after start")
	}
	lifecycle.RequireStop()
}

func TestServerStartFailsWhenPortIsOccupied(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	cfg := new(config.Config)
	cfg.Server.Port = listener.Addr().(*net.TCPAddr).Port
	lifecycle := fxtest.NewLifecycle(t)
	_, err = NewServer(ServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Logger:    newTestLoggerFactory(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if err := lifecycle.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want occupied-port error")
	} else if !strings.Contains(err.Error(), fmt.Sprintf(":%d", cfg.Server.Port)) {
		t.Fatalf("Start() error = %v, want address in error", err)
	}
}

func newTestLoggerFactory() logger.Factory {
	cfg := new(config.Config)
	return logger.NewFactory(logger.NewapiLogger(cfg))
}
