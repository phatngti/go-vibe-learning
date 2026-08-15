package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github/master_go_cqrs/internal/platform/security/gatewayauth"

	"github.com/labstack/echo/v4"
)

func TestV2HandlerRegistersPaymentAndRawWebhookRoutes(t *testing.T) {
	e := echo.New()
	handler := &V2Handler{authenticate: gatewayauth.NewMiddleware(), selectedProvider: "stripe"}
	handler.RegisterRoutes(e.Group("/api/v2/payments"))

	routes := make(map[string]struct{})
	for _, registered := range e.Routes() {
		routes[registered.Method+" "+registered.Path] = struct{}{}
	}
	for _, expected := range []string{
		"POST /api/v2/payments",
		"GET /api/v2/payments/:transaction_id",
		"POST /api/v2/payments/webhooks/stripe",
	} {
		if _, ok := routes[expected]; !ok {
			t.Errorf("missing route %q", expected)
		}
	}
}

func TestV2CreateRouteRequiresGatewayIdentity(t *testing.T) {
	e := echo.New()
	handler := &V2Handler{authenticate: gatewayauth.NewMiddleware(), selectedProvider: "stripe"}
	handler.RegisterRoutes(e.Group("/api/v2/payments"))

	request := httptest.NewRequest(http.MethodPost, "/api/v2/payments", nil)
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestStripeWebhookRouteIsDisabledForNonStripeProvider(t *testing.T) {
	e := echo.New()
	handler := &V2Handler{selectedProvider: "simulator"}
	request := httptest.NewRequest(http.MethodPost, "/api/v2/payments/webhooks/stripe", nil)
	recorder := httptest.NewRecorder()
	ctx := e.NewContext(request, recorder)
	if err := handler.StripeWebhook(ctx); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
