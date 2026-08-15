package gatewayauth

import (
	"net/http"
	"strings"
)

type RouteRule struct {
	Method   string
	Path     string
	Prefix   string
	Resource string
	Scopes   []string
}

type RouteAuthorizer struct {
	rules []RouteRule
}

func NewRouteAuthorizer(rules []RouteRule) RouteAuthorizer {
	return RouteAuthorizer{rules: rules}
}

func DefaultRouteAuthorizer() RouteAuthorizer {
	return NewRouteAuthorizer([]RouteRule{
		{Method: http.MethodPost, Path: "/api/v1/payment/initiate", Resource: "payment"},
		{Method: http.MethodPost, Path: "/api/v2/payments", Resource: "payment"},
		{Method: http.MethodGet, Prefix: "/api/v2/payments/", Resource: "payment"},
		{Prefix: "/api/v1/affiliate/", Resource: "affiliate"},
		{Prefix: "/api/v1/settlement/", Resource: "settlement"},
		{Prefix: "/api/v1/agent/", Resource: "agent"},
		{Method: http.MethodGet, Path: "/api/v1/auth/info", Resource: "auth"},
		{Method: http.MethodGet, Path: "/api/v1/auth/verify-token", Resource: "auth"},
		{Prefix: "/api/v1/author/", Resource: "access-control"},
	})
}

func (a RouteAuthorizer) Authorize(method, path string, user User) bool {
	for _, rule := range a.rules {
		if !rule.matches(method, path) {
			continue
		}
		if rule.Resource == "" || len(rule.Scopes) == 0 {
			return user.ID != ""
		}
		return HasResourceScopes(user.ResourceAccess, rule.Resource, rule.Scopes)
	}
	return false
}

func HasResourceScopes(resources []ResourceAccess, requiredResource string, requiredScopes []string) bool {
	for _, resource := range resources {
		if resource.ResourceName != requiredResource {
			continue
		}
		if len(requiredScopes) == 0 {
			return true
		}

		availableScopes := make(map[string]struct{}, len(resource.Scopes))
		for _, scope := range resource.Scopes {
			availableScopes[scope] = struct{}{}
		}
		for _, requiredScope := range requiredScopes {
			if _, ok := availableScopes[requiredScope]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func (r RouteRule) matches(method, path string) bool {
	if r.Method != "" && !strings.EqualFold(r.Method, method) {
		return false
	}
	if r.Path != "" {
		return path == r.Path
	}
	if r.Prefix != "" {
		return strings.HasPrefix(path, r.Prefix)
	}
	return false
}
