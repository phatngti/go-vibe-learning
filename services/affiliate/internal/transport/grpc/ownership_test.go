package grpc

import (
	"context"
	"testing"

	affiliatev1 "github/master_go_cqrs/contracts/genproto/affiliate"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAffiliateEndpointsRejectClientSelectedAgent(t *testing.T) {
	ctx := gatewayauth.WithUser(context.Background(), gatewayauth.User{
		ID: "subject-1", TenantID: "tenant-1", AgentID: "agent-owned",
	})
	handler := &ServerHandler{}
	tests := map[string]func() error{
		"register referral": func() error {
			_, err := handler.RegisterReferral(ctx, &affiliatev1.RegisterReferralRequest{ReferrerId: "agent-other"})
			return err
		},
		"get rank": func() error {
			_, err := handler.GetAgentRank(ctx, &affiliatev1.GetAgentRankRequest{AgentId: "agent-other"})
			return err
		},
		"get downline": func() error {
			_, err := handler.GetDownline(ctx, &affiliatev1.GetDownlineRequest{AgentId: "agent-other"})
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
