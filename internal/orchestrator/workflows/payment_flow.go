package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type PaymentProcessingPayload struct {
	TransactionID string  `json:"id"`
	AgentID       string  `json:"agent_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
}

func PaymentProcessingWorkflow(ctx workflow.Context, payloadStr string) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PaymentProcessingWorkflow", "payload", payloadStr)

	// Step 1: Accumulate Affiliate Downline Volume
	var affiliateRes string
	err := workflow.ExecuteActivity(ctx, "AddTransactionVolume", payloadStr).Get(ctx, &affiliateRes)
	if err != nil {
		logger.Error("Step 1 failed: AddTransactionVolume", "error", err)
		return err
	}

	// Step 2: Calculate and Credit Affiliate Commission
	var settlementRes string
	err = workflow.ExecuteActivity(ctx, "CalculateAndCreditCommission", payloadStr, affiliateRes).Get(ctx, &settlementRes)
	if err != nil {
		logger.Error("Step 2 failed: CalculateAndCreditCommission", "error", err)
		return err
	}

	// Step 3: Trigger Agent Callback Notification Webhook
	var agentRes string
	err = workflow.ExecuteActivity(ctx, "DispatchCallback", payloadStr).Get(ctx, &agentRes)
	if err != nil {
		logger.Error("Step 3 failed: DispatchCallback", "error", err)
		return err
	}

	logger.Info("PaymentProcessingWorkflow completed successfully")
	return nil
}
