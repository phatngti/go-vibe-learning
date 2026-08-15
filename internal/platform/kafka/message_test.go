package kafka

import (
	"context"
	"testing"
)

func TestPrepareMessagePopulatesV1AndV2Aliases(t *testing.T) {
	msg := &Message{
		EventID:          "event-1",
		EventType:        "UserSynced",
		AggregateID:      "user-1",
		AggregateVersion: 7,
		Payload:          []byte(`{"user_id":"user-1"}`),
		TraceParent:      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		TraceState:       "vendor=value",
	}

	carrier := PrepareMessage(context.Background(), msg)

	if msg.ID != "event-1" || msg.EventID != "event-1" {
		t.Fatalf("message identifiers = ID %q EventID %q", msg.ID, msg.EventID)
	}
	if msg.Version != 7 || msg.AggregateVersion != 7 {
		t.Fatalf("aggregate versions = legacy %d v2 %d", msg.Version, msg.AggregateVersion)
	}
	if msg.SchemaVersion != 1 || msg.CorrelationID != "user-1" {
		t.Fatalf("message metadata = schema %d correlation %q", msg.SchemaVersion, msg.CorrelationID)
	}
	if carrier.Get("traceparent") != msg.TraceParent || carrier.Get("tracestate") != msg.TraceState {
		t.Fatalf("trace carrier = %+v", carrier)
	}
}

func TestValidateMessageRequiresCompleteOrchestrationIdentity(t *testing.T) {
	base := Message{ID: "event-1", EventType: "PaymentCompleted", Payload: []byte(`{}`)}

	ownerOnly := base
	ownerOnly.OrchestrationOwner = "temporal:v2"
	if err := validateMessage(&ownerOnly); err == nil {
		t.Fatal("validateMessage(owner only) error = nil")
	}

	generationOnly := base
	generationOnly.OrchestrationGeneration = 2
	if err := validateMessage(&generationOnly); err == nil {
		t.Fatal("validateMessage(generation only) error = nil")
	}

	complete := base
	complete.OrchestrationOwner = "temporal:v2"
	complete.OrchestrationGeneration = 2
	if err := validateMessage(&complete); err != nil {
		t.Fatalf("validateMessage(complete) error = %v", err)
	}
}
