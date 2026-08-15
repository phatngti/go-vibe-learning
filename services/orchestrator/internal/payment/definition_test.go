package payment

import (
	"errors"
	"testing"

	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
)

func TestDefinitionTransitions(t *testing.T) {
	definition := NewDefinition()
	start, err := definition.Apply(nil, Event{
		Kind: EventPaymentCompleted, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDefinitionTransition(t, start, StateWaitingAffiliate, CommandAccumulateVolume, false)

	affiliate, err := definition.Apply(&start.Snapshot, Event{
		Kind: EventVolumeAccumulated, TransactionID: "tx-1", AgentID: "agent-1", Rank: "SILVER",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDefinitionTransition(t, affiliate, StateWaitingSettlement, CommandCreditCommission, false)

	settlement, err := definition.Apply(&affiliate.Snapshot, Event{
		Kind: EventCommissionCredited, TransactionID: "tx-1", AgentID: "agent-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDefinitionTransition(t, settlement, StateWaitingCallback, CommandDispatchCallback, false)

	callback, err := definition.Apply(&settlement.Snapshot, Event{
		Kind: EventCallbackCompleted, TransactionID: "tx-1", AgentID: "agent-1", CallbackStatus: "SKIPPED",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertDefinitionTransition(t, callback, StateCompleted, "", true)
}

func TestDefinitionDuplicateAndFutureEvents(t *testing.T) {
	definition := NewDefinition()
	snapshot := Snapshot{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", State: StateWaitingAffiliate}
	duplicate, err := definition.Apply(&snapshot, Event{
		Kind: EventPaymentCompleted, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err != nil || duplicate.Changed || duplicate.Command != nil {
		t.Fatalf("duplicate transition = %+v, error = %v", duplicate, err)
	}
	_, err = definition.Apply(&snapshot, Event{
		Kind: EventCommissionCredited, TransactionID: "tx-1", AgentID: "agent-1",
	})
	if !errors.Is(err, processcore.ErrNotReady) {
		t.Fatalf("future event error = %v", err)
	}
}

func TestDefinitionRejectsInvalidAndMissingStart(t *testing.T) {
	definition := NewDefinition()
	_, err := definition.Apply(nil, Event{Kind: EventVolumeAccumulated, TransactionID: "tx-1", AgentID: "agent-1", Rank: "SILVER"})
	if !errors.Is(err, processcore.ErrNotStarted) {
		t.Fatalf("missing start error = %v", err)
	}
	_, err = definition.Apply(nil, Event{Kind: EventPaymentCompleted})
	if !errors.Is(err, processcore.ErrInvalidEvent) {
		t.Fatalf("invalid event error = %v", err)
	}
}

func TestDefinitionTimeoutPolicy(t *testing.T) {
	definition := NewDefinition()
	states := []State{StateWaitingAffiliate, StateWaitingSettlement, StateWaitingCallback}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			snapshot := Snapshot{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Rank: "SILVER", State: state}
			retry, err := definition.OnTimeout(snapshot, MaxAttempts-1)
			if err != nil || retry.Command == nil || retry.Terminal {
				t.Fatalf("retry = %+v, error = %v", retry, err)
			}
			exhausted, err := definition.OnTimeout(snapshot, MaxAttempts)
			if err != nil || exhausted.Snapshot.State != StateNeedsManualReview || exhausted.Snapshot.ResumeState != state || exhausted.Terminal {
				t.Fatalf("exhausted = %+v, error = %v", exhausted, err)
			}
		})
	}
}

func TestDefinitionLateEventRecoversManualReview(t *testing.T) {
	definition := NewDefinition()
	review := Snapshot{
		TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
		State: StateNeedsManualReview, ResumeState: StateWaitingAffiliate,
	}
	transition, err := definition.Apply(&review, Event{
		Kind: EventVolumeAccumulated, TransactionID: "tx-1", AgentID: "agent-1", Rank: "SILVER",
	})
	if err != nil || transition.Snapshot.State != StateWaitingSettlement || transition.Snapshot.ResumeState != "" || transition.Command == nil {
		t.Fatalf("transition = %+v, error = %v", transition, err)
	}
}

func TestDefinitionDistinguishesDefinitiveAndUncertainPaymentOutcomes(t *testing.T) {
	definition := NewDefinition()
	waiting := Snapshot{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", State: StateWaitingPayment}
	failed, err := definition.Apply(&waiting, Event{
		Kind: EventPaymentFailed, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Reason: "declined",
	})
	if err != nil || failed.Snapshot.State != StateFailed || !failed.Terminal {
		t.Fatalf("failed = %+v, error = %v", failed, err)
	}
	uncertain, err := definition.Apply(&waiting, Event{
		Kind: EventPaymentUncertain, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Reason: "provider timeout",
	})
	if err != nil || uncertain.Snapshot.State != StateNeedsManualReview || uncertain.Terminal {
		t.Fatalf("uncertain = %+v, error = %v", uncertain, err)
	}
}

func TestDefinitionCallbackFailure(t *testing.T) {
	definition := NewDefinition()
	snapshot := Snapshot{TransactionID: "tx-1", AgentID: "agent-1", State: StateWaitingCallback}
	transition, err := definition.Apply(&snapshot, Event{
		Kind: EventCallbackCompleted, TransactionID: "tx-1", AgentID: "agent-1",
		CallbackStatus: "FAILED", CallbackReason: "timeout", CallbackFailed: true,
	})
	if err != nil || transition.Snapshot.State != StateCallbackFailed || transition.Snapshot.LastError != "timeout" {
		t.Fatalf("transition = %+v, error = %v", transition, err)
	}
}

func assertDefinitionTransition(t *testing.T, transition processcore.Transition[Snapshot, Command], state State, command CommandKind, terminal bool) {
	t.Helper()
	if transition.Snapshot.State != state || transition.Terminal != terminal {
		t.Fatalf("transition = %+v", transition)
	}
	if command == "" {
		if transition.Command != nil {
			t.Fatalf("command = %+v, want nil", transition.Command)
		}
		return
	}
	if transition.Command == nil || transition.Command.Kind != command {
		t.Fatalf("command = %+v, want %s", transition.Command, command)
	}
}
