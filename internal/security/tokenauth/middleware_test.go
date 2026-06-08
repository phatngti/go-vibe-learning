package tokenauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func TestAuthorizeFallsBackToAuthorizationHeader(t *testing.T) {
	cfg := new(config.Config)
	cfg.Keycloak.ClientSecret = "secret"
	token := signedToken(t, cfg.Keycloak.ClientSecret)

	if got := authorizeStatus(t, cfg, echo.HeaderAuthorization, "Bearer "+token); got != http.StatusNoContent {
		t.Fatalf("authorize status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestAuthorizeUsesConfiguredCustomHeader(t *testing.T) {
	cfg := new(config.Config)
	cfg.Keycloak.ClientSecret = "secret"
	cfg.Keycloak.CustomHeader = "X-Auth-Token"
	token := signedToken(t, cfg.Keycloak.ClientSecret)

	if got := authorizeStatus(t, cfg, cfg.Keycloak.CustomHeader, "Bearer "+token); got != http.StatusNoContent {
		t.Fatalf("authorize status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestAuthorizeWithResourceAllowsJWTResourceScopes(t *testing.T) {
	cfg := new(config.Config)
	cfg.Keycloak.ClientSecret = "secret"
	token := signedTokenWithClaims(t, cfg.Keycloak.ClientSecret, jwt.MapClaims{
		"sub": "user-id",
		"authorization": map[string]interface{}{
			"permissions": []map[string]interface{}{
				{
					"rsname": "orders",
					"scopes": []string{"read", "write"},
				},
			},
		},
	})

	got := authorizeWithResourceStatus(t, cfg, echo.HeaderAuthorization, "Bearer "+token, "orders", []string{"read"})
	if got != http.StatusNoContent {
		t.Fatalf("authorize status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestAuthorizeWithResourceRejectsMissingJWTResourceScope(t *testing.T) {
	cfg := new(config.Config)
	cfg.Keycloak.ClientSecret = "secret"
	token := signedTokenWithClaims(t, cfg.Keycloak.ClientSecret, jwt.MapClaims{
		"sub": "user-id",
		"authorization": map[string]interface{}{
			"permissions": []map[string]interface{}{
				{
					"rsname": "orders",
					"scopes": []string{"read"},
				},
			},
		},
	})

	got := authorizeWithResourceStatus(t, cfg, echo.HeaderAuthorization, "Bearer "+token, "orders", []string{"delete"})
	if got != http.StatusForbidden {
		t.Fatalf("authorize status = %d, want %d", got, http.StatusForbidden)
	}
}

func authorizeStatus(t *testing.T, cfg *config.Config, header, value string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(header, value)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	handler := NewMiddleware(MiddlewareParams{Config: cfg, Logger: logger.Factory{}}).Authorize()(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	if err := handler(ctx); err != nil {
		e.HTTPErrorHandler(err, ctx)
	}
	return rec.Code
}

func authorizeWithResourceStatus(t *testing.T, cfg *config.Config, header, value, resource string, scopes []string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(header, value)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	handler := NewMiddleware(MiddlewareParams{Config: cfg, Logger: logger.Factory{}}).AuthorizeWithResource(resource, scopes)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	if err := handler(ctx); err != nil {
		e.HTTPErrorHandler(err, ctx)
	}
	return rec.Code
}

func signedToken(t *testing.T, secret string) string {
	t.Helper()
	return signedTokenWithClaims(t, secret, jwt.MapClaims{"sub": "user-id"})
}

func signedTokenWithClaims(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}
