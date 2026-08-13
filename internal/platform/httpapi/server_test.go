package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx/fxtest"
)

type testRegistrar struct{}

func (testRegistrar) Pattern(*echo.Group) string { return "" }

func (testRegistrar) RegisterRoutes(group *echo.Group) {
	group.GET("/test", func(c echo.Context) error { return c.NoContent(204) })
}

func TestServerRoutes(t *testing.T) {
	lifecycle := fxtest.NewLifecycle(t)
	registrars := []route.Registrar{testRegistrar{}}

	server, err := NewServer(ServerParams{
		Lifecycle:    lifecycle,
		Server:       new(config.Server),
		LoggerConfig: new(config.Logger),
		Routes:       registrars,
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
		"GET /api/v1/test",
		"GET /internal/ext-authz/check/*",
	}

	for _, expectedRoute := range expected {
		if _, ok := routes[expectedRoute]; !ok {
			t.Errorf("missing route %q", expectedRoute)
		}
	}
}

func TestServerLifecycle(t *testing.T) {
	cfg := new(config.Server)
	cfg.Port = 0
	lifecycle := fxtest.NewLifecycle(t)
	server, err := NewServer(ServerParams{
		Lifecycle:    lifecycle,
		Server:       cfg,
		LoggerConfig: new(config.Logger),
		Logger:       newTestLoggerFactory(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.listen = func(string, string) (net.Listener, error) { return newBlockingListener(), nil }

	lifecycle.RequireStart()
	if server.echo.ListenerAddr() == nil {
		t.Fatal("ListenerAddr() = nil after start")
	}
	lifecycle.RequireStop()
}

func TestServerStartFailsWhenPortIsOccupied(t *testing.T) {
	cfg := new(config.Server)
	cfg.Port = 4321
	lifecycle := fxtest.NewLifecycle(t)
	server, err := NewServer(ServerParams{
		Lifecycle:    lifecycle,
		Server:       cfg,
		LoggerConfig: new(config.Logger),
		Logger:       newTestLoggerFactory(),
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.listen = func(_, address string) (net.Listener, error) {
		if address != ":4321" {
			t.Fatalf("address = %q", address)
		}
		return nil, errors.New("address already in use")
	}

	if err := lifecycle.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil, want occupied-port error")
	} else if !strings.Contains(err.Error(), fmt.Sprintf(":%d", cfg.Port)) {
		t.Fatalf("Start() error = %v, want address in error", err)
	}
}

type blockingListener struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingListener() *blockingListener { return &blockingListener{closed: make(chan struct{})} }

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *blockingListener) Addr() net.Addr { return testAddr(":0") }

type testAddr string

func (testAddr) Network() string  { return "tcp" }
func (a testAddr) String() string { return string(a) }

func newTestLoggerFactory() logger.Factory {
	cfg := new(config.Logger)
	return logger.NewFactory(logger.NewapiLogger(cfg))
}
