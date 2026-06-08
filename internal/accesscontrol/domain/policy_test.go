package domain

import "testing"

func TestHasResourceScopes(t *testing.T) {
	resources := []ResourceAccess{
		{ResourceName: "orders", Scopes: []string{"read", "write"}},
		{ResourceName: "reports", Scopes: nil},
	}

	tests := []struct {
		name     string
		resource string
		scopes   []string
		want     bool
	}{
		{name: "resource without explicit scopes", resource: "orders", want: true},
		{name: "all required scopes present", resource: "orders", scopes: []string{"read", "write"}, want: true},
		{name: "missing required scope", resource: "orders", scopes: []string{"delete"}, want: false},
		{name: "missing resource", resource: "payments", scopes: []string{"read"}, want: false},
		{name: "resource with no available scopes allows no required scopes", resource: "reports", want: true},
		{name: "resource with no available scopes denies required scope", resource: "reports", scopes: []string{"read"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasResourceScopes(resources, tt.resource, tt.scopes); got != tt.want {
				t.Fatalf("HasResourceScopes() = %v, want %v", got, tt.want)
			}
		})
	}
}
