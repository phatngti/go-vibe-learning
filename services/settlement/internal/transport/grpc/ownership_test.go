package grpc

import (
	"context"
	"testing"

	settlementv1 "github/master_go_cqrs/contracts/genproto/settlement"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSettlementEndpointsRejectClientSelectedAgent(t *testing.T) {
	ctx := gatewayauth.WithUser(context.Background(), gatewayauth.User{
		ID: "subject-1", TenantID: "tenant-1", AgentID: "agent-owned",
	})
	handler := &ServerHandler{}
	tests := map[string]func() error{
		"get balance": func() error {
			_, err := handler.GetAgentBalance(ctx, &settlementv1.GetAgentBalanceRequest{AgentId: "agent-other"})
			return err
		},
		"withdraw": func() error {
			_, err := handler.RequestWithdrawal(ctx, &settlementv1.RequestWithdrawalRequest{AgentId: "agent-other"})
			return err
		},
		"credit limit": func() error {
			_, err := handler.AdjustCreditLimit(ctx, &settlementv1.AdjustCreditLimitRequest{AgentId: "agent-other"})
			return err
		},
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			if err := run(); status.Code(err) != codes.PermissionDenied {
				t.Fatalf("error = %v, want PermissionDenied", err)
			}
		})
	}
}
