package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"

	kafkago "github.com/segmentio/kafka-go"
)

type capturingProducer struct{ messages []*Message }

func (p *capturingProducer) Publish(_ context.Context, _, _ string, message *Message) error {
	copy := *message
	p.messages = append(p.messages, &copy)
	return nil
}
func (*capturingProducer) Close() error { return nil }

func TestDeadLetterHasDeterministicIdentityAndBoundedEvidence(t *testing.T) {
	producer := new(capturingProducer)
	consumer := &consumer{producer: producer, deadLetterMaxPayloadBytes: 8}
	raw := kafkago.Message{
		Topic: "payments.events", Partition: 4, Offset: 91, Key: []byte("tx-1"),
		Value: []byte(`{"id":"message-that-is-long"}`),
	}

	for range 2 {
		if err := consumer.deadLetter(context.Background(), "orchestrator-payments", raw, "message-1", "validation", errTestPermanent); err != nil {
			t.Fatal(err)
		}
	}
	if len(producer.messages) != 2 || producer.messages[0].ID != producer.messages[1].ID {
		t.Fatalf("dead-letter IDs are not deterministic: %+v", producer.messages)
	}
	if producer.messages[0].ID != "dlq:orchestrator-payments:payments.events:4:91" {
		t.Fatalf("dead-letter ID = %q", producer.messages[0].ID)
	}
	var payload messagebus.DeadLetter
	if err := json.Unmarshal(producer.messages[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.RawMessageTruncated || len(payload.RawMessage) != 8 || payload.RawMessageSHA256 == "" {
		t.Fatalf("bounded DLQ evidence = %+v", payload)
	}
	if payload.SourceConsumer != "orchestrator-payments" || payload.SourcePartition != 4 || payload.SourceOffset != 91 {
		t.Fatalf("source coordinates = %+v", payload)
	}
}

var errTestPermanent = permanentError{err: testError("invalid event")}

type testError string

func (e testError) Error() string { return string(e) }

func TestKafkaCompressionValidation(t *testing.T) {
	if compression, err := kafkaCompression("lz4"); err != nil || compression != kafkago.Lz4 {
		t.Fatalf("kafkaCompression(lz4) = %v, %v", compression, err)
	}
	if _, err := kafkaCompression("made-up"); err == nil {
		t.Fatal("kafkaCompression(made-up) error = nil")
	}
}
