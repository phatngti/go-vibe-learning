package query

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/accesscontrol/application/dto"
	"github/master_go_cqrs/internal/accesscontrol/domain"
)

func TestCheckRBACHandler(t *testing.T) {
	handler := NewCheckRBACHandler()

	result := handler.Handle(context.Background(), domain.User{
		ResourceAccess: []domain.ResourceAccess{
			{ResourceName: "orders", Scopes: []string{"read"}},
		},
	}, dto.CheckRBACRequest{
		Resource: "orders",
		Scopes:   []string{"read"},
	})

	if !result.Allowed {
		t.Fatal("Allowed = false, want true")
	}
	if result.Resource != "orders" {
		t.Fatalf("Resource = %q, want orders", result.Resource)
	}
}
