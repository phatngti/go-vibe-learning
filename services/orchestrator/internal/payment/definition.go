package payment

import (
	"fmt"
	"time"

	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
)

const (
	StepTimeout = 30 * time.Second
	MaxAttempts = 5
)

type Definition struct{}

var _ processcore.Definition[Snapshot, Event, Command] = Definition{}

func NewDefinition() Definition { return Definition{} }

func (Definition) Policy() processcore.Policy {
	return processcore.Policy{StepTimeout: StepTimeout, MaxAttempts: MaxAttempts}
}

func (d Definition) Apply(current *Snapshot, event Event) (processcore.Transition[Snapshot, Command], error) {
	if err := ValidateEvent(event); err != nil {
		return processcore.Transition[Snapshot, Command]{}, err
	}
	if current == nil {
		switch event.Kind {
		case EventPaymentInitiated:
			monetaryAmount, _ := event.MonetaryAmount()
			legacyAmount, _ := legacyMajorFloat(monetaryAmount)
			snapshot := Snapshot{
				TransactionID: event.TransactionID, AgentID: event.AgentID,
				Money: moneyPointer(monetaryAmount), Amount: legacyAmount, Currency: monetaryAmount.Currency, State: StateWaitingPayment,
				OrchestrationOwner: event.OrchestrationOwner, OrchestrationGeneration: event.OrchestrationGeneration,
			}
			return transition(snapshot, nil, true), nil
		case EventPaymentCompleted:
			monetaryAmount, _ := event.MonetaryAmount()
			legacyAmount, _ := legacyMajorFloat(monetaryAmount)
			snapshot := Snapshot{
				TransactionID: event.TransactionID, AgentID: event.AgentID,
				Money: moneyPointer(monetaryAmount), Amount: legacyAmount, Currency: monetaryAmount.Currency, State: StateWaitingAffiliate,
				OrchestrationOwner: event.OrchestrationOwner, OrchestrationGeneration: event.OrchestrationGeneration,
			}
			return transition(snapshot, commandFor(snapshot), true), nil
		default:
			return processcore.Transition[Snapshot, Command]{}, processcore.NotStarted(event.TransactionID)
		}
	}
	if event.TransactionID != current.TransactionID || event.AgentID != current.AgentID {
		return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("event identity does not match process %s", current.TransactionID)
	}
	snapshot := *current
	if event.Kind == EventPaymentInitiated || event.Kind == EventPaymentCompleted || event.Kind == EventPaymentFailed || event.Kind == EventPaymentUncertain {
		currentMoney, err := snapshot.MonetaryAmount()
		if err != nil {
			return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("stored payment money is invalid: %v", err)
		}
		eventMoney, err := event.MonetaryAmount()
		if err != nil {
			return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("payment event money is invalid: %v", err)
		}
		if currentMoney != eventMoney {
			return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("payment event money conflicts with process %s", current.TransactionID)
		}
		snapshot.Money = moneyPointer(currentMoney)
		snapshot.Currency = currentMoney.Currency
		snapshot.Amount, _ = legacyMajorFloat(currentMoney)
	}
	if event.OrchestrationOwner != "" {
		if snapshot.OrchestrationOwner == "" {
			snapshot.OrchestrationOwner = event.OrchestrationOwner
			snapshot.OrchestrationGeneration = event.OrchestrationGeneration
		} else if event.OrchestrationOwner != snapshot.OrchestrationOwner || event.OrchestrationGeneration != snapshot.OrchestrationGeneration {
			return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("event orchestration owner does not match process %s", current.TransactionID)
		}
	}
	if snapshot.Terminal() {
		return transition(snapshot, nil, false), nil
	}
	if snapshot.State == StateNeedsManualReview {
		return d.applyLateOutcome(snapshot, event)
	}

	switch event.Kind {
	case EventPaymentInitiated:
		// Duplicate or late arrival; ignore.
		return transition(snapshot, nil, false), nil
	case EventPaymentCompleted:
		if snapshot.State == StateWaitingPayment {
			snapshot.State = StateWaitingAffiliate
			snapshot.LastError = ""
			return transition(snapshot, commandFor(snapshot), true), nil
		}
		return transition(snapshot, nil, false), nil
	case EventPaymentFailed:
		if snapshot.State == StateWaitingPayment {
			snapshot.State = StateFailed
			snapshot.LastError = event.Reason
			return transition(snapshot, nil, true), nil
		}
		// A failure that conflicts with already-applied downstream financial
		// effects needs investigation; it must not rewrite history as FAILED.
		snapshot.ResumeState = snapshot.State
		snapshot.State = StateNeedsManualReview
		snapshot.LastError = "payment failure conflicts with downstream progress: " + event.Reason
		return transition(snapshot, nil, true), nil
	case EventPaymentUncertain:
		if snapshot.State != StateWaitingPayment {
			return transition(snapshot, nil, false), nil
		}
		snapshot.ResumeState = snapshot.State
		snapshot.State = StateNeedsManualReview
		snapshot.LastError = event.Reason
		return transition(snapshot, nil, true), nil
	case EventVolumeAccumulated:
		if snapshot.State == StateWaitingPayment {
			return processcore.Transition[Snapshot, Command]{}, processcore.NotReady(snapshot.TransactionID, string(event.Kind))
		}
		if snapshot.State != StateWaitingAffiliate {
			return transition(snapshot, nil, false), nil
		}
		snapshot.State = StateWaitingSettlement
		snapshot.Rank = event.Rank
		snapshot.LastError = ""
		return transition(snapshot, commandFor(snapshot), true), nil
	case EventCommissionCredited:
		if snapshot.State == StateWaitingPayment || snapshot.State == StateWaitingAffiliate {
			return processcore.Transition[Snapshot, Command]{}, processcore.NotReady(snapshot.TransactionID, string(event.Kind))
		}
		if snapshot.State != StateWaitingSettlement {
			return transition(snapshot, nil, false), nil
		}
		snapshot.State = StateWaitingCallback
		snapshot.LastError = ""
		return transition(snapshot, commandFor(snapshot), true), nil
	case EventCallbackCompleted:
		if snapshot.State == StateWaitingPayment || snapshot.State == StateWaitingAffiliate || snapshot.State == StateWaitingSettlement {
			return processcore.Transition[Snapshot, Command]{}, processcore.NotReady(snapshot.TransactionID, string(event.Kind))
		}
		if snapshot.State != StateWaitingCallback {
			return transition(snapshot, nil, false), nil
		}
		if event.CallbackFailed {
			snapshot.State = StateCallbackFailed
			snapshot.LastError = event.CallbackReason
		} else {
			snapshot.State = StateCompleted
			snapshot.LastError = ""
		}
		return transition(snapshot, nil, true), nil
	default:
		return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("unsupported payment event %q", event.Kind)
	}
}

func (d Definition) OnTimeout(current Snapshot, attempt int) (processcore.Transition[Snapshot, Command], error) {
	if current.TransactionID == "" || current.Terminal() {
		return transition(current, nil, false), nil
	}
	if attempt >= d.Policy().MaxAttempts {
		current.ResumeState = current.State
		current.State = StateNeedsManualReview
		current.LastError = "process step exceeded retry limit; outcome is not definitive"
		return transition(current, nil, true), nil
	}
	// Waiting for a provider result has no command to reissue. The payment
	// service owns provider reconciliation and will publish a definitive event.
	if current.State == StateWaitingPayment {
		current.LastError = "payment result has not arrived"
		return transition(current, nil, false), nil
	}
	command := commandFor(current)
	if command == nil {
		return processcore.Transition[Snapshot, Command]{}, fmt.Errorf("payment process has no command for state %q", current.State)
	}
	current.LastError = "process step timed out"
	return transition(current, command, false), nil
}

func ValidateEvent(event Event) error {
	if event.TransactionID == "" || event.AgentID == "" {
		return processcore.Invalid("transaction_id and agent_id are required")
	}
	if (event.OrchestrationOwner == "") != (event.OrchestrationGeneration == 0) || event.OrchestrationGeneration < 0 {
		return processcore.Invalid("orchestration owner and positive generation must be provided together")
	}
	switch event.Kind {
	case EventPaymentInitiated:
		monetaryAmount, err := event.MonetaryAmount()
		if err != nil || !monetaryAmount.IsPositive() {
			return processcore.Invalid("payment amount and currency are required")
		}
	case EventPaymentCompleted:
		monetaryAmount, err := event.MonetaryAmount()
		if err != nil || !monetaryAmount.IsPositive() {
			return processcore.Invalid("payment amount and currency are required")
		}
	case EventPaymentFailed, EventPaymentUncertain:
		monetaryAmount, err := event.MonetaryAmount()
		if err != nil || !monetaryAmount.IsPositive() || event.Reason == "" {
			return processcore.Invalid("payment amount, currency, and outcome reason are required")
		}
	case EventVolumeAccumulated:
		if event.Rank == "" {
			return processcore.Invalid("affiliate rank is required")
		}
	case EventCommissionCredited:
	case EventCallbackCompleted:
		if event.CallbackStatus == "" {
			return processcore.Invalid("callback status is required")
		}
	default:
		return processcore.Invalid("unsupported payment event %q", event.Kind)
	}
	return nil
}

func (d Definition) applyLateOutcome(review Snapshot, event Event) (processcore.Transition[Snapshot, Command], error) {
	if review.ResumeState == "" || review.ResumeState == StateNeedsManualReview {
		return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("review state %s has no resumable step", review.TransactionID)
	}
	resumed := review
	resumed.State = review.ResumeState
	resumed.ResumeState = ""
	result, err := d.Apply(&resumed, event)
	if err != nil {
		return result, err
	}
	if !result.Changed && result.Command == nil && !result.Terminal {
		// Duplicate or stale events do not clear the operational review state.
		return transition(review, nil, false), nil
	}
	if result.Snapshot.State != StateNeedsManualReview {
		result.Snapshot.ResumeState = ""
		result.Snapshot.LastError = ""
	}
	return transition(result.Snapshot, result.Command, true), nil
}

func transition(snapshot Snapshot, command *Command, changed bool) processcore.Transition[Snapshot, Command] {
	return processcore.Transition[Snapshot, Command]{
		Snapshot: snapshot, Command: command, Changed: changed, Terminal: snapshot.Terminal(),
	}
}

func commandFor(snapshot Snapshot) *Command {
	command := &Command{
		TransactionID: snapshot.TransactionID, AgentID: snapshot.AgentID,
		Amount: snapshot.Amount, Currency: snapshot.Currency, Rank: snapshot.Rank,
		OrchestrationOwner: snapshot.OrchestrationOwner, OrchestrationGeneration: snapshot.OrchestrationGeneration,
	}
	if monetaryAmount, err := snapshot.MonetaryAmount(); err == nil {
		command.Money = moneyPointer(monetaryAmount)
		command.Currency = monetaryAmount.Currency
	}
	switch snapshot.State {
	case StateWaitingAffiliate:
		command.Kind = CommandAccumulateVolume
	case StateWaitingSettlement:
		command.Kind = CommandCreditCommission
	case StateWaitingCallback:
		command.Kind = CommandDispatchCallback
	default:
		return nil
	}
	return command
}
