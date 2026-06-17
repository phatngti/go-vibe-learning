package payment

import (
	"fmt"
	"time"

	processcore "github/master_go_cqrs/internal/orchestrator/core/process"
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
			snapshot := Snapshot{
				TransactionID: event.TransactionID, AgentID: event.AgentID,
				Amount: event.Amount, Currency: event.Currency, State: StateWaitingPayment,
			}
			return transition(snapshot, nil, true), nil
		case EventPaymentCompleted:
			snapshot := Snapshot{
				TransactionID: event.TransactionID, AgentID: event.AgentID,
				Amount: event.Amount, Currency: event.Currency, State: StateWaitingAffiliate,
			}
			return transition(snapshot, commandFor(snapshot), true), nil
		default:
			return processcore.Transition[Snapshot, Command]{}, processcore.NotStarted(event.TransactionID)
		}
	}
	if event.TransactionID != current.TransactionID || event.AgentID != current.AgentID {
		return processcore.Transition[Snapshot, Command]{}, processcore.Invalid("event identity does not match process %s", current.TransactionID)
	}
	if current.Terminal() {
		return transition(*current, nil, false), nil
	}

	snapshot := *current
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
	// StateWaitingPayment has no outbound command to retry; if the payment
	// does not complete within the retry budget the process is abandoned.
	if current.State == StateWaitingPayment {
		if attempt >= d.Policy().MaxAttempts {
			current.State = StateFailed
			current.LastError = "payment was not completed in time"
			return transition(current, nil, true), nil
		}
		return transition(current, nil, false), nil
	}
	if attempt >= d.Policy().MaxAttempts {
		if current.State == StateWaitingCallback {
			current.State = StateCallbackFailed
		} else {
			current.State = StateFailed
		}
		current.LastError = "process step exceeded retry limit"
		return transition(current, nil, true), nil
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
	switch event.Kind {
	case EventPaymentInitiated:
		if event.Amount <= 0 || event.Currency == "" {
			return processcore.Invalid("payment amount and currency are required")
		}
	case EventPaymentCompleted:
		if event.Amount <= 0 || event.Currency == "" {
			return processcore.Invalid("payment amount and currency are required")
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

func transition(snapshot Snapshot, command *Command, changed bool) processcore.Transition[Snapshot, Command] {
	return processcore.Transition[Snapshot, Command]{
		Snapshot: snapshot, Command: command, Changed: changed, Terminal: snapshot.Terminal(),
	}
}

func commandFor(snapshot Snapshot) *Command {
	command := &Command{
		TransactionID: snapshot.TransactionID, AgentID: snapshot.AgentID,
		Amount: snapshot.Amount, Currency: snapshot.Currency, Rank: snapshot.Rank,
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
