package gatewayauth

import "testing"

func TestDecodeUserPayloadMapsKeycloakClaims(t *testing.T) {
	payload, err := EncodeUserPayloadForTest(map[string]any{
		"sub":                "user-1",
		"preferred_username": "alice",
		"given_name":         "Alice",
		"family_name":        "Nguyen",
		"email":              "alice@example.com",
		"scope":              "openid profile",
		"aud":                []string{"master-go-cqrs"},
		"email_verified":     true,
		"realm_access": map[string]any{
			"roles": []string{"admin"},
		},
		"authorization": map[string]any{
			"permissions": []map[string]any{
				{"rsid": "resource-1", "rsname": "orders", "scopes": []string{"read", "write"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("EncodeUserPayloadForTest() error = %v", err)
	}

	user, err := DecodeUserPayload(payload)
	if err != nil {
		t.Fatalf("DecodeUserPayload() error = %v", err)
	}

	if user.ID != "user-1" || user.Username != "alice" || user.Audience != "master-go-cqrs" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Fatalf("roles = %#v, want admin", user.Roles)
	}
	if !HasResourceScopes(user.ResourceAccess, "orders", []string{"read"}) {
		t.Fatalf("resource access = %#v, want orders/read", user.ResourceAccess)
	}
}

func TestDecodeUserPayloadRejectsMalformedPayload(t *testing.T) {
	if _, err := DecodeUserPayload("not-valid-base64"); err == nil {
		t.Fatal("DecodeUserPayload() error = nil, want error")
	}
}
