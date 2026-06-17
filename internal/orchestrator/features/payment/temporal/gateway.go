package temporal

import (
	"context"
	"errors"
	"fmt"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/orchestrator/features/payment"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type WorkflowClient interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	SignalWorkflow(context.Context, string, string, string, interface{}) error
	DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
}

type WorkflowGateway interface {
	StartPayment(context.Context, payment.Event) error
	SignalPayment(context.Context, payment.Event) error
}

type Gateway struct {
	client    WorkflowClient
	taskQueue string
}

func NewGateway(c client.Client, cfg *config.Config) WorkflowGateway {
	return &Gateway{client: c, taskQueue: cfg.Orchestrator.Temporal.TaskQueue}
}

func (g *Gateway) StartPayment(ctx context.Context, event payment.Event) error {
	_, err := g.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: WorkflowID(event.TransactionID), TaskQueue: g.taskQueue,
		WorkflowIDReusePolicy:                    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy:                 enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowExecutionErrorWhenAlreadyStarted: false,
	}, PaymentWorkflowName, event)
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}

func (g *Gateway) SignalPayment(ctx context.Context, event payment.Event) error {
	workflowID := WorkflowID(event.TransactionID)
	err := g.client.SignalWorkflow(ctx, workflowID, "", ProcessEventSignalName, event)
	if err == nil {
		return nil
	}
	description, describeErr := g.client.DescribeWorkflowExecution(ctx, workflowID, "")
	if describeErr == nil && description.WorkflowExecutionInfo.GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return nil
	}
	var notFound *serviceerror.NotFound
	if describeErr != nil && !errors.As(describeErr, &notFound) {
		return fmt.Errorf("describe Temporal workflow %s after signal failure: %w", workflowID, describeErr)
	}
	return fmt.Errorf("signal Temporal workflow %s: %w", workflowID, err)
}
