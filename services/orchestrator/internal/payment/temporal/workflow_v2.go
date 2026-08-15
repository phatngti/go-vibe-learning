package temporal

import (
	"errors"
	"time"

	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"

	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const maxRememberedMessageIDs = 256

func paymentWorkflowV2(ctx workflow.Context, initial payment.Event) (WorkflowState, error) {
	definition := payment.NewDefinition()
	transition, err := definition.Apply(nil, initial)
	if err != nil {
		return WorkflowState{}, err
	}
	state := WorkflowState{Snapshot: transition.Snapshot}
	rememberMessage(&state, initial.MessageID)
	if err := workflow.SetQueryHandler(ctx, WorkflowStateQuery, func() (WorkflowState, error) { return state, nil }); err != nil {
		return state, err
	}

	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporalsdk.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: 30 * time.Second,
		},
	})
	signalChannel := workflow.GetSignalChannel(ctx, ProcessEventSignalName)
	pending := make([]payment.Event, 0, 4)

	for {
		transition, pending, err = applyReadyPendingV2(definition, transition, &state, pending)
		if err != nil {
			return state, err
		}
		if transition.Terminal {
			state.Snapshot = transition.Snapshot
			state.Attempt = 0
			return state, nil
		}
		if state.Snapshot.State == payment.StateNeedsManualReview {
			transition, pending, err = waitForRecoveryV2(ctx, signalChannel, definition, &state, pending)
			if err != nil {
				return state, err
			}
			continue
		}

		transition, pending, err = executeStepV2(ctx, activityCtx, signalChannel, definition, &state, transition.Command, pending)
		if err != nil {
			return state, err
		}
	}
}

func executeStepV2(
	ctx workflow.Context,
	activityCtx workflow.Context,
	signalChannel workflow.ReceiveChannel,
	definition payment.Definition,
	state *WorkflowState,
	command *payment.Command,
	pending []payment.Event,
) (processcore.Transition[payment.Snapshot, payment.Command], []payment.Event, error) {
	policy := definition.Policy()
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		state.Attempt = attempt
		if command != nil {
			// Infrastructure publication is retried by Temporal until it succeeds.
			// Business waiting starts only after Kafka acknowledged the command.
			if err := workflow.ExecuteActivity(activityCtx, PublishCommandActivityName, *command).Get(activityCtx, nil); err != nil {
				return processcore.Transition[payment.Snapshot, payment.Command]{Snapshot: state.Snapshot}, pending, err
			}
		}

		timerCtx, cancelTimer := workflow.WithCancel(ctx)
		timer := workflow.NewTimer(timerCtx, policy.StepTimeout)
		for {
			var received payment.Event
			receivedSignal := false
			timedOut := false
			selector := workflow.NewSelector(ctx)
			selector.AddReceive(signalChannel, func(channel workflow.ReceiveChannel, _ bool) {
				channel.Receive(ctx, &received)
				receivedSignal = true
			})
			selector.AddFuture(timer, func(workflow.Future) { timedOut = true })
			selector.Select(ctx)

			if timedOut {
				cancelTimer()
				transition, err := definition.OnTimeout(state.Snapshot, attempt)
				if err != nil {
					return transition, pending, err
				}
				state.Snapshot = transition.Snapshot
				if transition.Snapshot.State == payment.StateNeedsManualReview || transition.Terminal {
					return transition, pending, nil
				}
				command = transition.Command
				break
			}
			if !receivedSignal || rememberMessage(state, received.MessageID) {
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
				// Stale semantic duplicates are consumed.
			case errors.Is(applyErr, processcore.ErrNotReady):
				pending = append(pending, received)
			default:
				cancelTimer()
				return transition, pending, applyErr
			}
		}
	}
	return processcore.Transition[payment.Snapshot, payment.Command]{Snapshot: state.Snapshot}, pending, nil
}

func waitForRecoveryV2(
	ctx workflow.Context,
	signalChannel workflow.ReceiveChannel,
	definition payment.Definition,
	state *WorkflowState,
	pending []payment.Event,
) (processcore.Transition[payment.Snapshot, payment.Command], []payment.Event, error) {
	for {
		var received payment.Event
		signalChannel.Receive(ctx, &received)
		if rememberMessage(state, received.MessageID) {
			continue
		}
		transition, err := definition.Apply(&state.Snapshot, received)
		switch {
		case err == nil && (transition.Changed || transition.Command != nil || transition.Terminal):
			state.Snapshot = transition.Snapshot
			state.Attempt = 0
			return transition, pending, nil
		case err == nil:
			// Keep waiting for an authoritative event that resolves the review.
		case errors.Is(err, processcore.ErrNotReady):
			pending = append(pending, received)
		default:
			return transition, pending, err
		}
	}
}

func applyReadyPendingV2(
	definition payment.Definition,
	current processcore.Transition[payment.Snapshot, payment.Command],
	state *WorkflowState,
	pending []payment.Event,
) (processcore.Transition[payment.Snapshot, payment.Command], []payment.Event, error) {
	for len(pending) > 0 {
		transition, remaining, applied, err := applyPending(definition, state.Snapshot, pending)
		if err != nil {
			return transition, remaining, err
		}
		pending = remaining
		if !applied {
			break
		}
		current = transition
		state.Snapshot = transition.Snapshot
		state.Attempt = 0
	}
	return current, pending, nil
}

// rememberMessage returns true for a duplicate. The bounded list is persisted
// in workflow history/state and supplements business idempotency at consumers.
func rememberMessage(state *WorkflowState, messageID string) bool {
	if messageID == "" {
		return false
	}
	for _, seen := range state.SeenMessageIDs {
		if seen == messageID {
			return true
		}
	}
	state.SeenMessageIDs = append(state.SeenMessageIDs, messageID)
	if len(state.SeenMessageIDs) > maxRememberedMessageIDs {
		state.SeenMessageIDs = append([]string(nil), state.SeenMessageIDs[len(state.SeenMessageIDs)-maxRememberedMessageIDs:]...)
	}
	return false
}
