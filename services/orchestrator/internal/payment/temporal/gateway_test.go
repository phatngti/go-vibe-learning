package temporal

import (
	"context"
	"errors"
	"testing"

	"github/master_go_cqrs/services/orchestrator/internal/payment"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type workflowClientStub struct {
	startOptions client.StartWorkflowOptions
	workflow     interface{}
	startErr     error
	signalErr    error
	status       enumspb.WorkflowExecutionStatus
}

func (c *workflowClientStub) ExecuteWorkflow(_ context.Context, options client.StartWorkflowOptions, workflow interface{}, _ ...interface{}) (client.WorkflowRun, error) {
	c.startOptions = options
	c.workflow = workflow
	return nil, c.startErr
}

func (c *workflowClientStub) SignalWorkflow(context.Context, string, string, string, interface{}) error {
	return c.signalErr
}

func (c *workflowClientStub) DescribeWorkflowExecution(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	return &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: c.status},
	}, nil
}

func TestGatewayUsesDeterministicWorkflowIdentity(t *testing.T) {
	stub := &workflowClientStub{}
	gateway := &Gateway{client: stub, taskQueue: "payment-orchestration"}
	if err := gateway.StartPayment(context.Background(), payment.Event{Kind: payment.EventPaymentCompleted, TransactionID: "tx-1"}); err != nil {
		t.Fatal(err)
	}
	if stub.startOptions.ID != "payment:tx-1" || stub.startOptions.TaskQueue != "payment-orchestration" {
		t.Fatalf("start options = %+v", stub.startOptions)
	}
	if stub.startOptions.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE ||
		stub.startOptions.WorkflowIDConflictPolicy != enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING {
		t.Fatalf("deduplication policies = %+v", stub.startOptions)
	}
	if stub.workflow != PaymentWorkflowName {
		t.Fatalf("workflow = %v", stub.workflow)
	}
}

func TestGatewayAcceptsSignalRedeliveryAfterWorkflowClosed(t *testing.T) {
	stub := &workflowClientStub{signalErr: errors.New("workflow already completed"), status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED}
	gateway := &Gateway{client: stub, taskQueue: "payment-orchestration"}
	if err := gateway.SignalPayment(context.Background(), payment.Event{Kind: payment.EventVolumeAccumulated, TransactionID: "tx-1"}); err != nil {
		t.Fatalf("SignalPayment() error = %v", err)
	}
}

func TestGatewayAcceptsDuplicateWorkflowStart(t *testing.T) {
	stub := &workflowClientStub{startErr: &serviceerror.WorkflowExecutionAlreadyStarted{Message: "duplicate", RunId: "run-1"}}
	gateway := &Gateway{client: stub, taskQueue: "payment-orchestration"}
	if err := gateway.StartPayment(context.Background(), payment.Event{Kind: payment.EventPaymentCompleted, TransactionID: "tx-1"}); err != nil {
		t.Fatalf("StartPayment() error = %v", err)
	}
}

func TestGatewayRetriesSignalFailureForRunningWorkflow(t *testing.T) {
	stub := &workflowClientStub{signalErr: errors.New("temporal unavailable"), status: enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING}
	gateway := &Gateway{client: stub, taskQueue: "payment-orchestration"}
	if err := gateway.SignalPayment(context.Background(), payment.Event{Kind: payment.EventVolumeAccumulated, TransactionID: "tx-1"}); err == nil {
		t.Fatal("SignalPayment() error = nil")
	}
}
