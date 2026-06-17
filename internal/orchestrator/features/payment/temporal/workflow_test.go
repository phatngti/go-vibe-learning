package temporal

import (
	"testing"
	"time"

	"github/master_go_cqrs/internal/orchestrator/features/payment"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func TestPaymentWorkflowCompletesWithDelayedOutOfOrderSignals(t *testing.T) {
	env, published := newWorkflowEnvironment(t)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCommissionCredited, TransactionID: "tx-1", AgentID: "agent-1",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		event := payment.Event{Kind: payment.EventVolumeAccumulated, TransactionID: "tx-1", AgentID: "agent-1", Rank: "SILVER"}
		env.SignalWorkflow(ProcessEventSignalName, event)
		env.SignalWorkflow(ProcessEventSignalName, event)
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCallbackCompleted, TransactionID: "tx-1", AgentID: "agent-1", CallbackStatus: "SKIPPED",
		})
	}, 3*time.Second)

	env.ExecuteWorkflow(PaymentWorkflow, payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result WorkflowState
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.State != payment.StateCompleted {
		t.Fatalf("state = %q, want completed", result.Snapshot.State)
	}
	if len(*published) != 3 {
		t.Fatalf("published commands = %d, want 3", len(*published))
	}
}

func TestPaymentWorkflowCallbackFailureIsTerminal(t *testing.T) {
	env, _ := newWorkflowEnvironment(t)
	signalHappyFinancialPath(env, "tx-2", "agent-2")
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCallbackCompleted, TransactionID: "tx-2", AgentID: "agent-2",
			CallbackStatus: "FAILED", CallbackReason: "timeout", CallbackFailed: true,
		})
	}, 3*time.Second)

	env.ExecuteWorkflow(PaymentWorkflow, payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-2", AgentID: "agent-2", Amount: 100, Currency: "USD",
	})
	var result WorkflowState
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.State != payment.StateCallbackFailed || result.Snapshot.LastError != "timeout" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPaymentWorkflowStopsAfterFiveTimedOutCommands(t *testing.T) {
	env, published := newWorkflowEnvironment(t)
	env.ExecuteWorkflow(PaymentWorkflow, payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-3", AgentID: "agent-3", Amount: 100, Currency: "USD",
	})
	var result WorkflowState
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.State != payment.StateFailed || result.Attempt != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(*published) != payment.MaxAttempts {
		t.Fatalf("published commands = %d, want %d", len(*published), payment.MaxAttempts)
	}
}

func TestPaymentWorkflowRejectsMalformedSignal(t *testing.T) {
	env, _ := newWorkflowEnvironment(t)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventVolumeAccumulated, TransactionID: "tx-invalid", Rank: "SILVER",
		})
	}, time.Second)
	env.ExecuteWorkflow(PaymentWorkflow, payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-invalid", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("GetWorkflowError() = nil")
	}
}

func TestPaymentWorkflowStateQueryUsesSharedSnapshot(t *testing.T) {
	env, _ := newWorkflowEnvironment(t)
	var queried WorkflowState
	env.RegisterDelayedCallback(func() {
		value, err := env.QueryWorkflow(WorkflowStateQuery)
		if err != nil {
			t.Fatalf("QueryWorkflow() error = %v", err)
		}
		if err := value.Get(&queried); err != nil {
			t.Fatalf("query result error = %v", err)
		}
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventVolumeAccumulated, TransactionID: "tx-query", AgentID: "agent-1", Rank: "SILVER",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCommissionCredited, TransactionID: "tx-query", AgentID: "agent-1",
		})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCallbackCompleted, TransactionID: "tx-query", AgentID: "agent-1", CallbackStatus: "SKIPPED",
		})
	}, 3*time.Second)
	env.ExecuteWorkflow(PaymentWorkflow, payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-query", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if queried.Snapshot.State != payment.StateWaitingAffiliate || queried.Attempt != 1 {
		t.Fatalf("queried state = %+v", queried)
	}
}

func TestPaymentWorkflowReplaysExistingStartHistory(t *testing.T) {
	input, err := converter.GetDefaultDataConverter().ToPayloads(payment.Event{
		Kind: payment.EventPaymentCompleted, TransactionID: "tx-replay", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	history := &historypb.History{Events: []*historypb.HistoryEvent{
		{
			EventId:   1,
			EventType: enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
			Attributes: &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{
				WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
					WorkflowType: &commonpb.WorkflowType{Name: PaymentWorkflowName},
					TaskQueue:    &taskqueuepb.TaskQueue{Name: "payment-orchestration"},
					Input:        input,
				},
			},
		},
		{
			EventId:   2,
			EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_SCHEDULED,
			Attributes: &historypb.HistoryEvent_WorkflowTaskScheduledEventAttributes{
				WorkflowTaskScheduledEventAttributes: &historypb.WorkflowTaskScheduledEventAttributes{},
			},
		},
		{EventId: 3, EventType: enumspb.EVENT_TYPE_WORKFLOW_TASK_STARTED},
	}}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(PaymentWorkflow, workflow.RegisterOptions{Name: PaymentWorkflowName})
	if err := replayer.ReplayWorkflowHistory(nil, history); err != nil {
		t.Fatalf("ReplayWorkflowHistory() error = %v", err)
	}
}

func newWorkflowEnvironment(t *testing.T) (*testsuite.TestWorkflowEnvironment, *[]payment.Command) {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	published := make([]payment.Command, 0, 3)
	env.RegisterActivityWithOptions(func(command payment.Command) error {
		published = append(published, command)
		return nil
	}, activity.RegisterOptions{Name: PublishCommandActivityName})
	return env, &published
}

func signalHappyFinancialPath(env *testsuite.TestWorkflowEnvironment, transactionID, agentID string) {
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventVolumeAccumulated, TransactionID: transactionID, AgentID: agentID, Rank: "BRONZE",
		})
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ProcessEventSignalName, payment.Event{
			Kind: payment.EventCommissionCredited, TransactionID: transactionID, AgentID: agentID,
		})
	}, 2*time.Second)
}
