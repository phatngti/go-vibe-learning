package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github/master_go_cqrs/internal/accesscontrol/application/dto"
	"github/master_go_cqrs/internal/accesscontrol/application/query"
	"github/master_go_cqrs/internal/httpapi/response"
	"github/master_go_cqrs/internal/middleware"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type testAuth struct {
	user gatewayauth.User
}

func (a testAuth) Authorize() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user", a.user)
			return next(c)
		}
	}
}

func (a testAuth) AuthorizeWithResource(string, []string) echo.MiddlewareFunc {
	return a.Authorize()
}

func TestCheckAuthnHandlerReturnsAuthenticatedUser(t *testing.T) {
	e := newTestEcho(testAuth{user: gatewayauth.User{ID: "user-1", Username: "alice"}})
	req := httptest.NewRequest("GET", "/author/authn/check", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Code != dto.AuthnCheckSuccess {
		t.Fatalf("code = %q, want %q", got.Code, dto.AuthnCheckSuccess)
	}
}

func TestCheckRBACHandlerAllowsResourceScopes(t *testing.T) {
	e := newTestEcho(testAuth{user: gatewayauth.User{
		ResourceAccess: []gatewayauth.ResourceAccess{{ResourceName: "orders", Scopes: []string{"read", "write"}}},
	}})
	req := httptest.NewRequest("POST", "/author/rbac/check", strings.NewReader(`{"resource":"orders","scopes":["read"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Code != dto.RBACCheckSuccess {
		t.Fatalf("code = %q, want %q", got.Code, dto.RBACCheckSuccess)
	}
}

func TestCheckRBACHandlerDeniesMissingScope(t *testing.T) {
	e := newTestEcho(testAuth{user: gatewayauth.User{
		ResourceAccess: []gatewayauth.ResourceAccess{{ResourceName: "orders", Scopes: []string{"read"}}},
	}})
	req := httptest.NewRequest("POST", "/author/rbac/check", strings.NewReader(`{"resource":"orders","scopes":["delete"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}

	var got response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.Code != dto.RBACCheckFailed {
		t.Fatalf("code = %q, want %q", got.Code, dto.RBACCheckFailed)
	}
}

func TestCheckRBACHandlerValidatesResource(t *testing.T) {
	e := newTestEcho(testAuth{})
	req := httptest.NewRequest("POST", "/author/rbac/check", strings.NewReader(`{"scopes":["read"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), dto.InvalidResourceName) {
		t.Fatalf("body = %s, want validation code %q", rec.Body.String(), dto.InvalidResourceName)
	}
}

func newTestEcho(auth gatewayauth.Middleware) *echo.Echo {
	e := echo.New()
	e.Validator = &middleware.CustomValidator{Validator: validator.New()}
	NewHandler(HandlerParams{
		CheckAuthn: query.NewCheckAuthnHandler(),
		CheckRBAC:  query.NewCheckRBACHandler(),
		TokenAuth:  auth,
	}).RegisterRoutes(e.Group("/author"))
	return e
}
