package grpc

import (
	"context"
	"testing"

	paymentv1 "github/master_go_cqrs/contracts/genproto/payment"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInitiatePaymentRejectsClientSelectedAgent(t *testing.T) {
	ctx := gatewayauth.WithUser(context.Background(), gatewayauth.User{
		ID: "subject-1", TenantID: "tenant-1", AgentID: "agent-owned",
	})
	_, err := (&ServerHandler{}).InitiatePayment(ctx, &paymentv1.InitiatePaymentRequest{AgentId: "agent-other"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("error = %v, want PermissionDenied", err)
	}
}
