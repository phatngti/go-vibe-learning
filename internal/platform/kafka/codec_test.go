package kafka

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type stubSchemaRegistry struct {
	id          int
	schema      RegistrySchema
	registerErr error
	lookupErr   error
	subject     string
}

func (s *stubSchemaRegistry) Register(_ context.Context, subject string, schema RegistrySchema) (SchemaRegistration, error) {
	s.subject = subject
	s.schema = schema
	if s.registerErr != nil {
		return SchemaRegistration{}, s.registerErr
	}
	return SchemaRegistration{ID: s.id, Version: 1}, nil
}

func (s *stubSchemaRegistry) Lookup(_ context.Context, _ string, _ int) (RegistrySchema, error) {
	if s.lookupErr != nil {
		return RegistrySchema{}, s.lookupErr
	}
	return s.schema, nil
}

func TestProtobufEnvelopeCodecUsesConfluentWireFormatAndRoundTrips(t *testing.T) {
	registry := &stubSchemaRegistry{id: 73}
	codec, err := NewProtobufEnvelopeCodec(registry)
	if err != nil {
		t.Fatal(err)
	}
	registry.schema = codec.schema
	when := time.Date(2026, time.August, 14, 9, 30, 0, 123_000_000, time.UTC)
	message := &Message{
		SchemaVersion: 2, EventID: "event-1", EventType: "PaymentCompleted",
		AggregateType: "Transaction", AggregateID: "payment-1", AggregateVersion: 9,
		Payload: []byte(`{"transaction_id":"payment-1"}`), OccurredAt: when,
		TenantID: "tenant-1", CorrelationID: "correlation-1", CausationID: "command-1",
		OrchestrationOwner: "temporal:v2", OrchestrationGeneration: 2,
		TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		TraceState:  "vendor=value",
	}

	wire, err := codec.Encode(context.Background(), "payments.events.v2", message)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) < 7 || wire[0] != 0 || binary.BigEndian.Uint32(wire[1:5]) != 73 || wire[5] != 0 {
		t.Fatalf("unexpected Confluent wire header: %v", wire[:min(len(wire), 6)])
	}
	if registry.subject != "payments.events.v2-value" {
		t.Fatalf("subject = %q", registry.subject)
	}

	decoded, err := codec.Decode(context.Background(), "payments.events.v2", wire)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != message.EventID || decoded.EventID != message.EventID || decoded.EventType != message.EventType {
		t.Fatalf("decoded identity = %+v", decoded)
	}
	if decoded.AggregateID != message.AggregateID || decoded.AggregateVersion != message.AggregateVersion || decoded.Version != message.AggregateVersion {
		t.Fatalf("decoded aggregate metadata = %+v", decoded)
	}
	if !decoded.OccurredAt.Equal(when) || !reflect.DeepEqual(decoded.Payload, message.Payload) {
		t.Fatalf("decoded timestamp/payload = %s %s", decoded.OccurredAt, decoded.Payload)
	}
	if decoded.TenantID != message.TenantID || decoded.OrchestrationOwner != message.OrchestrationOwner || decoded.TraceParent != message.TraceParent {
		t.Fatalf("decoded production metadata = %+v", decoded)
	}
	if decoded.ProtobufPayload == nil || !decoded.ProtobufPayload.MessageIs(&wrapperspb.BytesValue{}) {
		t.Fatalf("decoded protobuf payload = %+v", decoded.ProtobufPayload)
	}
}

func TestProtobufEnvelopeCodecPreservesTypedAnyPayload(t *testing.T) {
	registry := &stubSchemaRegistry{id: 9}
	codec, err := NewProtobufEnvelopeCodec(registry)
	if err != nil {
		t.Fatal(err)
	}
	registry.schema = codec.schema
	typed, err := anypb.New(wrapperspb.String("typed-domain-event"))
	if err != nil {
		t.Fatal(err)
	}
	message := &Message{
		ID: "event-typed", EventType: "TypedEvent", AggregateID: "aggregate-1",
		OccurredAt: time.Now().UTC(), ProtobufPayload: typed,
	}
	wire, err := codec.Encode(context.Background(), "typed.events.v2", message)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Decode(context.Background(), "typed.events.v2", wire)
	if err != nil {
		t.Fatal(err)
	}
	var value wrapperspb.StringValue
	if err := decoded.ProtobufPayload.UnmarshalTo(&value); err != nil {
		t.Fatal(err)
	}
	if value.Value != "typed-domain-event" {
		t.Fatalf("typed payload = %q", value.Value)
	}
}

func TestProtobufEnvelopeCodecFailsClosed(t *testing.T) {
	message := &Message{
		ID: "event-1", EventType: "PaymentCompleted", AggregateID: "payment-1",
		Payload: []byte(`{}`), OccurredAt: time.Now().UTC(),
	}

	registerFailure := &stubSchemaRegistry{id: 1, registerErr: errors.New("registry unavailable")}
	codec, err := NewProtobufEnvelopeCodec(registerFailure)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Encode(context.Background(), "payments.events.v2", message); err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Fatalf("Encode() error = %v", err)
	}

	lookupFailure := &stubSchemaRegistry{id: 1, schema: codec.schema, lookupErr: errors.New("unknown schema")}
	codec, err = NewProtobufEnvelopeCodec(lookupFailure)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := (&ProtobufEnvelopeCodec{registry: &stubSchemaRegistry{id: 1, schema: codec.schema}, schema: codec.schema}).Encode(
		context.Background(), "payments.events.v2", message,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Decode(context.Background(), "payments.events.v2", wire); err == nil || !strings.Contains(err.Error(), "unknown schema") {
		t.Fatalf("Decode() error = %v", err)
	}
}

func TestNewMessageCodecDefaultsToLegacyJSON(t *testing.T) {
	codec, err := NewMessageCodec(&platformconfig.Kafka{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := codec.(JSONMessageCodec); !ok {
		t.Fatalf("codec type = %T, want JSONMessageCodec", codec)
	}
	data, err := codec.Encode(context.Background(), "payments.events", &Message{ID: "event-1", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[0] != '{' {
		t.Fatalf("legacy JSON payload = %q", data)
	}
}

func TestEventEnvelopeV2SchemaMatchesContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	contractPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "contracts", "proto", "event_envelope_v2.proto")
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(contract)) != strings.TrimSpace(EventEnvelopeV2Schema) {
		t.Fatal("EventEnvelopeV2Schema drifted from contracts/proto/event_envelope_v2.proto")
	}
}

func TestDecodeMessageIndexes(t *testing.T) {
	indexes, consumed, err := decodeMessageIndexes([]byte{0, 99})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(indexes, []int{0}) || consumed != 1 {
		t.Fatalf("optimized indexes = %v consumed=%d", indexes, consumed)
	}
	if _, _, err := decodeMessageIndexes([]byte{0x80}); err == nil {
		t.Fatal("decodeMessageIndexes(truncated) error = nil")
	}
}
