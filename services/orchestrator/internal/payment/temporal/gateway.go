package temporal

import (
	"context"
	"errors"
	"fmt"

	"github/master_go_cqrs/services/orchestrator/internal/config"
	"github/master_go_cqrs/services/orchestrator/internal/payment"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type WorkflowClient interface {
	ExecuteWorkflow(context.Context, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	SignalWithStartWorkflow(context.Context, string, string, interface{}, client.StartWorkflowOptions, interface{}, ...interface{}) (client.WorkflowRun, error)
	SignalWorkflow(context.Context, string, string, string, interface{}) error
	DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error)
}

type WorkflowGateway interface {
	DeliverPayment(context.Context, payment.Event) error
}

var ErrWorkflowClosed = errors.New("payment workflow is closed")

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

// DeliverPayment atomically signals the deterministic workflow or starts it
// with the same event. This closes the race where PaymentInitiated starts a
// workflow and PaymentCompleted is then mistaken for a duplicate start rather
// than delivered as a state transition.
func (g *Gateway) DeliverPayment(ctx context.Context, event payment.Event) error {
	workflowID := WorkflowID(event.TransactionID)
	if event.Kind != payment.EventPaymentInitiated && event.Kind != payment.EventPaymentCompleted {
		// Downstream events are not valid workflow starts. Retrying until the
		// initiated/completed event creates the workflow preserves cross-topic
		// causal order without inventing an incomplete initial snapshot.
		return g.SignalPayment(ctx, event)
	}
	_, err := g.client.SignalWithStartWorkflow(ctx, workflowID, ProcessEventSignalName, event, client.StartWorkflowOptions{
		ID: workflowID, TaskQueue: g.taskQueue,
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}, PaymentWorkflowName, event)
	if err == nil {
		return nil
	}
	return g.classifyDeliveryFailure(ctx, workflowID, err)
}

func (g *Gateway) SignalPayment(ctx context.Context, event payment.Event) error {
	workflowID := WorkflowID(event.TransactionID)
	err := g.client.SignalWorkflow(ctx, workflowID, "", ProcessEventSignalName, event)
	if err == nil {
		return nil
	}
	description, describeErr := g.client.DescribeWorkflowExecution(ctx, workflowID, "")
	if describeErr == nil && description.WorkflowExecutionInfo.GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return fmt.Errorf("%w: %s has status %s", ErrWorkflowClosed, workflowID, description.WorkflowExecutionInfo.GetStatus())
	}
	var notFound *serviceerror.NotFound
	if describeErr != nil && !errors.As(describeErr, &notFound) {
		return fmt.Errorf("describe Temporal workflow %s after signal failure: %w", workflowID, describeErr)
	}
	return fmt.Errorf("signal Temporal workflow %s: %w", workflowID, err)
}

func (g *Gateway) classifyDeliveryFailure(ctx context.Context, workflowID string, deliveryErr error) error {
	description, describeErr := g.client.DescribeWorkflowExecution(ctx, workflowID, "")
	if describeErr == nil && description.WorkflowExecutionInfo.GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
		return fmt.Errorf("%w: %s has status %s: %v", ErrWorkflowClosed, workflowID, description.WorkflowExecutionInfo.GetStatus(), deliveryErr)
	}
	var notFound *serviceerror.NotFound
	if describeErr != nil && !errors.As(describeErr, &notFound) {
		return fmt.Errorf("describe Temporal workflow %s after delivery failure: %w", workflowID, describeErr)
	}
	return fmt.Errorf("deliver event to Temporal workflow %s: %w", workflowID, deliveryErr)
}
