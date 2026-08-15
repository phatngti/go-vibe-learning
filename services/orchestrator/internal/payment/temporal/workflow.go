package temporal

import (
	"errors"
	"fmt"
	"time"

	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const paymentWorkflowV2ChangeID = "payment-orchestration-v2"

// PaymentWorkflow keeps existing histories deterministic while routing new
// executions to the recoverable v2 state machine.
func PaymentWorkflow(ctx workflow.Context, initial payment.Event) (WorkflowState, error) {
	version := workflow.GetVersion(ctx, paymentWorkflowV2ChangeID, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return paymentWorkflowV1(ctx, initial)
	}
	return paymentWorkflowV2(ctx, initial)
}

func paymentWorkflowV1(ctx workflow.Context, initial payment.Event) (WorkflowState, error) {
	definition := payment.NewDefinition()
	transition, err := definition.Apply(nil, initial)
	if err != nil {
		return WorkflowState{}, err
	}
	state := WorkflowState{Snapshot: transition.Snapshot}
	if err := workflow.SetQueryHandler(ctx, WorkflowStateQuery, func() (WorkflowState, error) { return state, nil }); err != nil {
		return state, err
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	signalChannel := workflow.GetSignalChannel(ctx, ProcessEventSignalName)
	pending := make([]payment.Event, 0, 4)

	for !transition.Terminal {
		if transition.Command == nil {
			return state, fmt.Errorf("payment workflow state %q has no command", state.Snapshot.State)
		}
		transition, pending, err = executeStepV1(ctx, activityCtx, signalChannel, definition, &state, *transition.Command, pending)
		if err != nil {
			return state, err
		}
	}
	state.Snapshot = transition.Snapshot
	state.Attempt = 0
	return state, nil
}

func executeStepV1(
	ctx workflow.Context,
	activityCtx workflow.Context,
	signalChannel workflow.ReceiveChannel,
	definition payment.Definition,
	state *WorkflowState,
	command payment.Command,
	pending []payment.Event,
) (processcore.Transition[payment.Snapshot, payment.Command], []payment.Event, error) {
	policy := definition.Policy()
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		state.Attempt = attempt
		if err := workflow.ExecuteActivity(activityCtx, PublishCommandActivityName, command).Get(activityCtx, nil); err != nil {
			transition, transitionErr := legacyOnTimeout(definition, state.Snapshot, attempt)
			if transitionErr != nil {
				return transition, pending, transitionErr
			}
			state.Snapshot = transition.Snapshot
			if transition.Terminal {
				return transition, pending, nil
			}
			command = *transition.Command
			continue
		}

		if transition, remaining, applied, err := applyPending(definition, state.Snapshot, pending); err != nil {
			return transition, remaining, err
		} else if applied {
			state.Snapshot = transition.Snapshot
			state.Attempt = 0
			return transition, remaining, nil
		} else {
			pending = remaining
		}

		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, policy.StepTimeout)
		timedOut := false
		for !timedOut {
			var received payment.Event
			receivedSignal := false
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(signalChannel, func(channel workflow.ReceiveChannel, _ bool) {
				channel.Receive(ctx, &received)
				receivedSignal = true
			})
			selector.AddFuture(timer, func(workflow.Future) { timedOut = true })
			selector.Select(ctx)
			if !receivedSignal {
				continue
			}
			transition, applyErr := definition.Apply(&state.Snapshot, received)
			switch {
			case applyErr == nil && (transition.Changed || transition.Command != nil || transition.Terminal):
				cancelTimer()
				state.Snapshot = transition.Snapshot
				state.Attempt = 0
				return transition, pending, nil
			case applyErr == nil:
				// Duplicate or stale events are consumed without changing the process.
			case errors.Is(applyErr, processcore.ErrNotReady):
				pending = append(pending, received)
			default:
				cancelTimer()
				return transition, pending, applyErr
			}
		}
		cancelTimer()
		transition, transitionErr := legacyOnTimeout(definition, state.Snapshot, attempt)
		if transitionErr != nil {
			return transition, pending, transitionErr
		}
		state.Snapshot = transition.Snapshot
		if transition.Terminal {
			return transition, pending, nil
		}
		command = *transition.Command
	}
	return processcore.Transition[payment.Snapshot, payment.Command]{}, pending, fmt.Errorf("payment workflow exhausted attempts without terminal transition")
}

func legacyOnTimeout(definition payment.Definition, current payment.Snapshot, attempt int) (processcore.Transition[payment.Snapshot, payment.Command], error) {
	if current.TransactionID == "" || current.Terminal() {
		return legacyTransition(current, nil), nil
	}
	if current.State == payment.StateWaitingPayment {
		if attempt >= definition.Policy().MaxAttempts {
			current.State = payment.StateFailed
			current.LastError = "payment was not completed in time"
			return legacyTransition(current, nil), nil
		}
		return legacyTransition(current, nil), nil
	}
	if attempt >= definition.Policy().MaxAttempts {
		if current.State == payment.StateWaitingCallback {
			current.State = payment.StateCallbackFailed
		} else {
			current.State = payment.StateFailed
		}
		current.LastError = "process step exceeded retry limit"
		return legacyTransition(current, nil), nil
	}
	command := legacyCommandFor(current)
	if command == nil {
		return processcore.Transition[payment.Snapshot, payment.Command]{}, fmt.Errorf("payment process has no command for state %q", current.State)
	}
	current.LastError = "process step timed out"
	return legacyTransition(current, command), nil
}

func legacyTransition(snapshot payment.Snapshot, command *payment.Command) processcore.Transition[payment.Snapshot, payment.Command] {
	return processcore.Transition[payment.Snapshot, payment.Command]{
		Snapshot: snapshot, Command: command, Changed: true, Terminal: snapshot.Terminal(),
	}
}

func legacyCommandFor(snapshot payment.Snapshot) *payment.Command {
	command := &payment.Command{
		TransactionID: snapshot.TransactionID, AgentID: snapshot.AgentID,
		Amount: snapshot.Amount, Currency: snapshot.Currency, Rank: snapshot.Rank,
		OrchestrationOwner: snapshot.OrchestrationOwner, OrchestrationGeneration: snapshot.OrchestrationGeneration,
	}
	switch snapshot.State {
	case payment.StateWaitingAffiliate:
		command.Kind = payment.CommandAccumulateVolume
	case payment.StateWaitingSettlement:
		command.Kind = payment.CommandCreditCommission
	case payment.StateWaitingCallback:
		command.Kind = payment.CommandDispatchCallback
	default:
		return nil
	}
	return command
}

func applyPending(
	definition payment.Definition,
	snapshot payment.Snapshot,
	pending []payment.Event,
) (processcore.Transition[payment.Snapshot, payment.Command], []payment.Event, bool, error) {
	remaining := make([]payment.Event, 0, len(pending))
	for index, event := range pending {
		transition, err := definition.Apply(&snapshot, event)
		switch {
		case err == nil && (transition.Changed || transition.Command != nil || transition.Terminal):
			remaining = append(remaining, pending[index+1:]...)
			return transition, remaining, true, nil
		case err == nil:
			// Drop duplicate events.
		case errors.Is(err, processcore.ErrNotReady):
			remaining = append(remaining, event)
		default:
			return transition, remaining, false, err
		}
	}
	return processcore.Transition[payment.Snapshot, payment.Command]{Snapshot: snapshot}, remaining, false, nil
}
