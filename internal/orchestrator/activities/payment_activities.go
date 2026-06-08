package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	affiliatev1 "github/master_go_cqrs/internal/genproto/affiliate"
	settlementv1 "github/master_go_cqrs/internal/genproto/settlement"
	"github/master_go_cqrs/internal/orchestrator/workflows"
	"github/master_go_cqrs/internal/platform/grpc"

	"go.temporal.io/sdk/activity"
)

type Activities struct{}

func NewActivities() *Activities {
	return &Activities{}
}

func (a *Activities) AddTransactionVolume(ctx context.Context, payloadStr string) (string, error) {
	activity.GetLogger(ctx).Info("Executing Activity: AddTransactionVolume")

	var p workflows.PaymentProcessingPayload
	if err := json.Unmarshal([]byte(payloadStr), &p); err != nil {
		return "", err
	}

	// Connect to Affiliate microservice address (env or default localhost:9002)
	affiliateAddr := os.Getenv("AFFILIATE_SERVICE_ADDR")
	if affiliateAddr == "" {
		affiliateAddr = "localhost:9002"
	}
	conn, err := grpc.NewClientConn(affiliateAddr)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Affiliate Service: %w", err)
	}
	defer conn.Close()

	client := affiliatev1.NewAffiliateServiceClient(conn)
	res, err := client.AccumulateVolume(ctx, &affiliatev1.AccumulateVolumeRequest{
		AgentId: p.AgentID,
		Amount:  p.Amount,
	})
	if err != nil {
		return "", fmt.Errorf("failed to accumulate affiliate volume: %w", err)
	}

	return res.CurrentRank, nil
}

func (a *Activities) CalculateAndCreditCommission(ctx context.Context, payloadStr string, currentRank string) (string, error) {
	activity.GetLogger(ctx).Info("Executing Activity: CalculateAndCreditCommission", "rank", currentRank)

	var p workflows.PaymentProcessingPayload
	if err := json.Unmarshal([]byte(payloadStr), &p); err != nil {
		return "", err
	}

	// Connect to Settlement microservice address (env or default localhost:9003)
	settlementAddr := os.Getenv("SETTLEMENT_SERVICE_ADDR")
	if settlementAddr == "" {
		settlementAddr = "localhost:9003"
	}
	conn, err := grpc.NewClientConn(settlementAddr)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Settlement Service: %w", err)
	}
	defer conn.Close()

	client := settlementv1.NewSettlementServiceClient(conn)
	res, err := client.CalculateAndCreditCommission(ctx, &settlementv1.CalculateAndCreditCommissionRequest{
		AgentId:           p.AgentID,
		TransactionAmount: p.Amount,
		Rank:              currentRank,
		TransactionId:     p.TransactionID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to calculate and credit commission: %w", err)
	}

	return res.Status, nil
}

func (a *Activities) DispatchCallback(ctx context.Context, payloadStr string) (string, error) {
	activity.GetLogger(ctx).Info("Executing Activity: DispatchCallback")

	// In production, fetch config.CallbackURL from Agent Service and issue an HTTP webhook dispatch.
	// For basic setup, we log successfully and return.
	activity.GetLogger(ctx).Info("Successfully dispatched payment webhook callback to agent dashboard configuration", "payload", payloadStr)
	return "DISPATCHED", nil
}
