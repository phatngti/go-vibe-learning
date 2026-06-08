package gatewayauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHTTPMiddlewareSetsUser(t *testing.T) {
	payload := testPayload(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(PayloadHeader, payload)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := NewMiddleware().Authorize()(func(c echo.Context) error {
		user, ok := c.Get("user").(User)
		if !ok || user.ID != "user-1" {
			t.Fatalf("user = %#v, want user-1", c.Get("user"))
		}
		return c.NoContent(http.StatusNoContent)
	})(c)
	if err != nil {
		t.Fatalf("middleware error = %v", err)
	}
}

func TestExtAuthzCheck(t *testing.T) {
	payload := testPayload(t)
	handler := ExtAuthzCheck("/internal/ext-authz/check", NewRouteAuthorizer([]RouteRule{
		{Method: http.MethodGet, Path: "/api/v1/orders", Resource: "orders", Scopes: []string{"read"}},
	}))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/internal/ext-authz/check/api/v1/orders", nil)
	req.Header.Set(PayloadHeader, payload)
	rec := httptest.NewRecorder()
	if err := handler(e.NewContext(req, rec)); err != nil {
		t.Fatalf("ExtAuthzCheck() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func testPayload(t *testing.T) string {
	t.Helper()
	payload, err := EncodeUserPayloadForTest(map[string]any{
		"sub":                "user-1",
		"preferred_username": "alice",
		"authorization": map[string]any{
			"permissions": []map[string]any{
				{"rsname": "orders", "scopes": []string{"read"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
