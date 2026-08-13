package query

import (
	"context"
	"testing"

	"github/master_go_cqrs/services/user/internal/accesscontrol/domain"
)

func TestCheckAuthnHandler(t *testing.T) {
	handler := NewCheckAuthnHandler()

	result := handler.Handle(context.Background(), domain.User{ID: "user-1"})

	if !result.Authenticated {
		t.Fatal("Authenticated = false, want true")
	}
	if result.User.ID != "user-1" {
		t.Fatalf("User.ID = %q, want user-1", result.User.ID)
	}
}
