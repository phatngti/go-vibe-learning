package kafka

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	eventv2 "github/master_go_cqrs/contracts/genproto/eventv2"
	platformconfig "github/master_go_cqrs/internal/platform/config"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	confluentMagicByte = byte(0)

	// EventEnvelopeV2Schema is registered per <topic>-value subject. Keep this
	// source in lockstep with contracts/proto/event_envelope_v2.proto; a drift
	// test guards the package/message/import contract used by the codec.
	EventEnvelopeV2Schema = `syntax = "proto3";

package event.v2;

import "google/protobuf/any.proto";
import "google/protobuf/timestamp.proto";

option go_package = "github/master_go_cqrs/contracts/genproto/eventv2;eventv2";

// EventEnvelope is the production Kafka envelope for v2 integration topics.
// Payload is an Any so each event can retain a strongly typed Protobuf
// contract while sharing stable delivery, tenancy, orchestration, and tracing
// metadata.
message EventEnvelope {
  string event_id = 1;
  string event_type = 2;
  uint32 schema_version = 3;
  google.protobuf.Timestamp occurred_at = 4;
  string tenant_id = 5;
  string aggregate_id = 6;
  int64 aggregate_version = 7;
  string correlation_id = 8;
  string causation_id = 9;
  string orchestration_owner = 10;
  int64 orchestration_generation = 11;
  string trace_parent = 12;
  string trace_state = 13;
  google.protobuf.Any payload = 14;

  // Aggregate type is retained to preserve the routing/debugging metadata in
  // the legacy JSON envelope during the v1-to-v2 migration.
  string aggregate_type = 15;
}`

	protobufAnySchema = `syntax = "proto3";
package google.protobuf;
option go_package = "google.golang.org/protobuf/types/known/anypb";
message Any {
  string type_url = 1;
  bytes value = 2;
}`

	protobufTimestampSchema = `syntax = "proto3";
package google.protobuf;
option go_package = "google.golang.org/protobuf/types/known/timestamppb";
message Timestamp {
  int64 seconds = 1;
  int32 nanos = 2;
}`
)

type MessageCodec interface {
	Encode(ctx context.Context, topic string, msg *Message) ([]byte, error)
	Decode(ctx context.Context, topic string, data []byte) (*Message, error)
}

type JSONMessageCodec struct{}

func (JSONMessageCodec) Encode(_ context.Context, _ string, msg *Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("kafka: marshal JSON message: %w", err)
	}
	return data, nil
}

func (JSONMessageCodec) Decode(_ context.Context, _ string, data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("kafka: unmarshal JSON message: %w", err)
	}
	return &msg, nil
}

type ProtobufEnvelopeCodec struct {
	registry SchemaRegistry
	schema   RegistrySchema
}

func NewProtobufEnvelopeCodec(registry SchemaRegistry) (*ProtobufEnvelopeCodec, error) {
	if registry == nil {
		return nil, fmt.Errorf("kafka: schema registry is required for protobuf serialization")
	}
	return &ProtobufEnvelopeCodec{
		registry: registry,
		schema:   RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"},
	}, nil
}

func NewMessageCodec(cfg *platformconfig.Kafka) (MessageCodec, error) {
	if cfg == nil {
		return nil, fmt.Errorf("kafka: configuration is required")
	}
	if err := cfg.ValidateSerialization(); err != nil {
		return nil, err
	}
	format := strings.ToLower(strings.TrimSpace(cfg.Serialization.Format))
	if format == "" || format == platformconfig.KafkaSerializationJSON {
		return JSONMessageCodec{}, nil
	}
	registry, err := NewSchemaRegistryClient(cfg.SchemaRegistry)
	if err != nil {
		return nil, err
	}
	return NewProtobufEnvelopeCodec(registry)
}

func (c *ProtobufEnvelopeCodec) Encode(ctx context.Context, topic string, msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("kafka: message is required")
	}
	normalizeMessageAliases(msg)
	if err := validateMessage(msg); err != nil {
		return nil, err
	}
	payload, err := protobufPayload(msg)
	if err != nil {
		return nil, err
	}
	if msg.SchemaVersion < 0 || uint64(msg.SchemaVersion) > math.MaxUint32 {
		return nil, fmt.Errorf("kafka: schema version %d is outside uint32 range", msg.SchemaVersion)
	}
	occurredAt := timestamppb.New(msg.OccurredAt)
	if err := occurredAt.CheckValid(); err != nil {
		return nil, fmt.Errorf("kafka: invalid occurred_at: %w", err)
	}
	envelope := &eventv2.EventEnvelope{
		EventId: msg.EventID, EventType: msg.EventType,
		SchemaVersion: uint32(msg.SchemaVersion), OccurredAt: occurredAt,
		TenantId: msg.TenantID, AggregateId: msg.AggregateID,
		AggregateVersion: msg.AggregateVersion, CorrelationId: msg.CorrelationID,
		CausationId: msg.CausationID, OrchestrationOwner: msg.OrchestrationOwner,
		OrchestrationGeneration: msg.OrchestrationGeneration,
		TraceParent:             msg.TraceParent, TraceState: msg.TraceState,
		Payload: payload, AggregateType: msg.AggregateType,
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("kafka: marshal protobuf envelope: %w", err)
	}
	subject, err := valueSubject(topic)
	if err != nil {
		return nil, err
	}
	registeredSchema, err := c.schemaWithReferences(ctx)
	if err != nil {
		return nil, err
	}
	registration, err := c.registry.Register(ctx, subject, registeredSchema)
	if err != nil {
		return nil, fmt.Errorf("kafka: resolve protobuf schema for %q: %w", topic, err)
	}
	if registration.ID <= 0 || registration.ID > math.MaxInt32 {
		return nil, fmt.Errorf("kafka: registry returned invalid schema id %d", registration.ID)
	}

	// Confluent Protobuf framing is magic byte + schema id + message-index
	// array + payload. The sole top-level EventEnvelope has indexes [0], whose
	// canonical optimized encoding is one zero byte.
	wire := make([]byte, 6, 6+len(encoded))
	wire[0] = confluentMagicByte
	binary.BigEndian.PutUint32(wire[1:5], uint32(registration.ID))
	wire[5] = 0
	wire = append(wire, encoded...)
	return wire, nil
}

func (c *ProtobufEnvelopeCodec) schemaWithReferences(ctx context.Context) (RegistrySchema, error) {
	anyRegistration, err := c.registry.Register(ctx, "google/protobuf/any.proto", RegistrySchema{
		Schema: protobufAnySchema, SchemaType: "PROTOBUF",
	})
	if err != nil {
		return RegistrySchema{}, fmt.Errorf("kafka: register google/protobuf/any.proto: %w", err)
	}
	timestampRegistration, err := c.registry.Register(ctx, "google/protobuf/timestamp.proto", RegistrySchema{
		Schema: protobufTimestampSchema, SchemaType: "PROTOBUF",
	})
	if err != nil {
		return RegistrySchema{}, fmt.Errorf("kafka: register google/protobuf/timestamp.proto: %w", err)
	}
	if anyRegistration.Version <= 0 || timestampRegistration.Version <= 0 {
		return RegistrySchema{}, fmt.Errorf("kafka: registry returned invalid protobuf reference versions")
	}
	schema := c.schema
	schema.References = []SchemaReference{
		{Name: "google/protobuf/any.proto", Subject: "google/protobuf/any.proto", Version: anyRegistration.Version},
		{Name: "google/protobuf/timestamp.proto", Subject: "google/protobuf/timestamp.proto", Version: timestampRegistration.Version},
	}
	return schema, nil
}

func (c *ProtobufEnvelopeCodec) Decode(ctx context.Context, topic string, data []byte) (*Message, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("kafka: confluent protobuf payload is shorter than 6-byte header")
	}
	if data[0] != confluentMagicByte {
		return nil, fmt.Errorf("kafka: unsupported confluent magic byte %d", data[0])
	}
	schemaID := binary.BigEndian.Uint32(data[1:5])
	if schemaID == 0 || schemaID > math.MaxInt32 {
		return nil, fmt.Errorf("kafka: invalid schema id %d", schemaID)
	}
	subject, err := valueSubject(topic)
	if err != nil {
		return nil, err
	}
	registered, err := c.registry.Lookup(ctx, subject, int(schemaID))
	if err != nil {
		return nil, fmt.Errorf("kafka: lookup protobuf schema for %q: %w", topic, err)
	}
	if strings.ToUpper(registered.SchemaType) != "PROTOBUF" {
		return nil, fmt.Errorf("kafka: schema id %d is type %q, want PROTOBUF", schemaID, registered.SchemaType)
	}
	indexes, consumed, err := decodeMessageIndexes(data[5:])
	if err != nil {
		return nil, err
	}
	if len(indexes) != 1 || indexes[0] != 0 {
		return nil, fmt.Errorf("kafka: unsupported protobuf message indexes %v", indexes)
	}
	payloadOffset := 5 + consumed
	if payloadOffset >= len(data) {
		return nil, fmt.Errorf("kafka: confluent protobuf envelope payload is empty")
	}
	var envelope eventv2.EventEnvelope
	if err := proto.Unmarshal(data[payloadOffset:], &envelope); err != nil {
		return nil, fmt.Errorf("kafka: unmarshal protobuf envelope: %w", err)
	}
	return envelopeToMessage(&envelope)
}

func protobufPayload(msg *Message) (*anypb.Any, error) {
	if msg.ProtobufPayload != nil {
		cloned, ok := proto.Clone(msg.ProtobufPayload).(*anypb.Any)
		if !ok || cloned == nil {
			return nil, fmt.Errorf("kafka: clone protobuf payload")
		}
		if strings.TrimSpace(cloned.TypeUrl) == "" {
			return nil, fmt.Errorf("kafka: protobuf payload type URL is required")
		}
		return cloned, nil
	}
	if len(msg.Payload) == 0 {
		return nil, fmt.Errorf("kafka: payload is required")
	}
	wrapped, err := anypb.New(&wrapperspb.BytesValue{Value: msg.Payload})
	if err != nil {
		return nil, fmt.Errorf("kafka: wrap legacy payload: %w", err)
	}
	return wrapped, nil
}

func envelopeToMessage(envelope *eventv2.EventEnvelope) (*Message, error) {
	if envelope == nil {
		return nil, fmt.Errorf("kafka: protobuf envelope is required")
	}
	if envelope.OccurredAt == nil {
		return nil, fmt.Errorf("kafka: protobuf occurred_at is required")
	}
	if err := envelope.OccurredAt.CheckValid(); err != nil {
		return nil, fmt.Errorf("kafka: invalid protobuf occurred_at: %w", err)
	}
	if envelope.Payload == nil || strings.TrimSpace(envelope.Payload.TypeUrl) == "" {
		return nil, fmt.Errorf("kafka: typed protobuf payload is required")
	}
	protoPayload, ok := proto.Clone(envelope.Payload).(*anypb.Any)
	if !ok || protoPayload == nil {
		return nil, fmt.Errorf("kafka: clone protobuf envelope payload")
	}
	payload := protoPayload.Value
	if protoPayload.MessageIs(&wrapperspb.BytesValue{}) {
		var wrapped wrapperspb.BytesValue
		if err := protoPayload.UnmarshalTo(&wrapped); err != nil {
			return nil, fmt.Errorf("kafka: unwrap legacy payload: %w", err)
		}
		payload = wrapped.Value
	}
	msg := &Message{
		SchemaVersion: int(envelope.SchemaVersion),
		ID:            envelope.EventId, EventID: envelope.EventId,
		EventType: envelope.EventType, AggregateType: envelope.AggregateType,
		AggregateID: envelope.AggregateId, AggregateVersion: envelope.AggregateVersion,
		Version: envelope.AggregateVersion, OccurredAt: envelope.OccurredAt.AsTime(),
		TenantID: envelope.TenantId, CorrelationID: envelope.CorrelationId,
		CausationID: envelope.CausationId, OrchestrationOwner: envelope.OrchestrationOwner,
		OrchestrationGeneration: envelope.OrchestrationGeneration,
		TraceParent:             envelope.TraceParent, TraceState: envelope.TraceState,
		Payload: payload, ProtobufPayload: protoPayload,
	}
	normalizeMessageAliases(msg)
	if err := validateMessage(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func valueSubject(topic string) (string, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return "", fmt.Errorf("kafka: topic is required for schema subject")
	}
	return topic + "-value", nil
}

func decodeMessageIndexes(data []byte) ([]int, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("kafka: protobuf message indexes are missing")
	}
	value, read := binary.Uvarint(data)
	if read <= 0 {
		return nil, 0, fmt.Errorf("kafka: invalid protobuf message-index length")
	}
	if value == 0 {
		return []int{0}, read, nil
	}
	length := decodeZigZag(value)
	if length <= 0 || length > 100 {
		return nil, 0, fmt.Errorf("kafka: invalid protobuf message-index count %d", length)
	}
	indexes := make([]int, 0, length)
	offset := read
	for range length {
		if offset >= len(data) {
			return nil, 0, fmt.Errorf("kafka: truncated protobuf message indexes")
		}
		encoded, consumed := binary.Uvarint(data[offset:])
		if consumed <= 0 {
			return nil, 0, fmt.Errorf("kafka: invalid protobuf message index")
		}
		index := decodeZigZag(encoded)
		if index < 0 {
			return nil, 0, fmt.Errorf("kafka: protobuf message index cannot be negative")
		}
		indexes = append(indexes, index)
		offset += consumed
	}
	return indexes, offset, nil
}

func decodeZigZag(value uint64) int {
	decoded := int64(value >> 1)
	if value&1 != 0 {
		decoded = ^decoded
	}
	return int(decoded)
}
