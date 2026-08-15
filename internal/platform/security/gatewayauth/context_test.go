package gatewayauth

import (
	"context"
	"errors"
	"testing"
)

func TestRequireAgentOwnershipUsesVerifiedPrincipal(t *testing.T) {
	ctx := WithUser(context.Background(), User{ID: "user-1", TenantID: "tenant-1", AgentID: "agent-1"})

	ownership, err := RequireAgentOwnership(ctx, "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if ownership.TenantID != "tenant-1" || ownership.AgentID != "agent-1" {
		t.Fatalf("ownership = %+v", ownership)
	}
}

func TestRequireAgentOwnershipRejectsClientSelectedOwner(t *testing.T) {
	ctx := WithUser(context.Background(), User{ID: "user-1", TenantID: "tenant-1", AgentID: "agent-1"})

	_, err := RequireAgentOwnership(ctx, "agent-2")
	if !errors.Is(err, ErrOwnershipMismatch) {
		t.Fatalf("error = %v, want ErrOwnershipMismatch", err)
	}
}

func TestRequireAgentOwnershipRequiresTenantAndAgentClaims(t *testing.T) {
	_, err := RequireAgentOwnership(WithUser(context.Background(), User{ID: "user-1"}), "")
	if !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("error = %v, want ErrTenantRequired", err)
	}

	_, err = RequireAgentOwnership(WithUser(context.Background(), User{ID: "user-1", TenantID: "tenant-1"}), "")
	if !errors.Is(err, ErrAgentRequired) {
		t.Fatalf("error = %v, want ErrAgentRequired", err)
	}
}
