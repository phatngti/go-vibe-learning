package gatewayauth

import (
	"net/http"
	"testing"
)

func TestRouteAuthorizer(t *testing.T) {
	authorizer := NewRouteAuthorizer([]RouteRule{
		{Method: http.MethodGet, Path: "/protected", Resource: "orders", Scopes: []string{"read"}},
	})
	user := User{ID: "user-1", ResourceAccess: []ResourceAccess{
		{ResourceName: "orders", Scopes: []string{"read"}},
	}}

	if !authorizer.Authorize(http.MethodGet, "/protected", user) {
		t.Fatal("Authorize() = false, want true")
	}
	if authorizer.Authorize(http.MethodPost, "/protected", user) {
		t.Fatal("Authorize() = true for wrong method, want false")
	}
	if authorizer.Authorize(http.MethodGet, "/protected", User{ID: "user-1"}) {
		t.Fatal("Authorize() = true without required scope, want false")
	}
}

func TestDefaultRouteAuthorizerProtectsPaymentV2(t *testing.T) {
	authorizer := DefaultRouteAuthorizer()
	user := User{ID: "user-1"}

	if !authorizer.Authorize(http.MethodPost, "/api/v2/payments", user) {
		t.Fatal("payment v2 creation must allow an authenticated principal")
	}
	if !authorizer.Authorize(http.MethodGet, "/api/v2/payments/tx-1", user) {
		t.Fatal("payment v2 query must allow an authenticated principal")
	}
	if authorizer.Authorize(http.MethodGet, "/api/v2/payments", user) {
		t.Fatal("payment v2 collection GET is not a public contract")
	}
}
