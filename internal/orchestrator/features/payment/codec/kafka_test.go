package codec

import (
	"encoding/json"
	"errors"
	"testing"

	"github/master_go_cqrs/internal/config"
	processcore "github/master_go_cqrs/internal/orchestrator/core/process"
	"github/master_go_cqrs/internal/orchestrator/features/payment"
	platformkafka "github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
)

func TestKafkaCodecRoundTripsProcessContracts(t *testing.T) {
	codec := NewKafka(codecTestConfig())
	tests := []struct {
		command     payment.Command
		messageType string
		eventKind   payment.EventKind
		result      any
	}{
		{
			command:     payment.Command{Kind: payment.CommandAccumulateVolume, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD"},
			messageType: messagebus.AccumulateVolumeRequestedType, eventKind: payment.EventVolumeAccumulated,
			result: messagebus.VolumeAccumulated{TransactionID: "tx-1", AgentID: "agent-1", CurrentRank: "SILVER"},
		},
		{
			command:     payment.Command{Kind: payment.CommandCreditCommission, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Rank: "SILVER"},
			messageType: messagebus.CreditCommissionRequestedType, eventKind: payment.EventCommissionCredited,
			result: messagebus.CommissionCredited{TransactionID: "tx-1", AgentID: "agent-1"},
		},
		{
			command:     payment.Command{Kind: payment.CommandDispatchCallback, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD"},
			messageType: messagebus.DispatchCallbackRequestedType, eventKind: payment.EventCallbackCompleted,
			result: messagebus.CallbackResult{TransactionID: "tx-1", AgentID: "agent-1", Status: "SKIPPED"},
		},
	}
	for _, test := range tests {
		encoded, err := codec.Encode(test.command)
		if err != nil {
			t.Fatal(err)
		}
		if encoded.Message.EventType != test.messageType || encoded.Message.ID == "" || !json.Valid(encoded.Message.Payload) {
			t.Fatalf("encoded = %+v", encoded)
		}
		resultPayload, _ := json.Marshal(test.result)
		resultType := map[payment.EventKind]string{
			payment.EventVolumeAccumulated:  messagebus.VolumeAccumulatedType,
			payment.EventCommissionCredited: messagebus.CommissionCreditedType,
			payment.EventCallbackCompleted:  messagebus.CallbackDispatchSkippedType,
		}[test.eventKind]
		event, err := codec.Decode(&platformkafka.Message{EventType: resultType, Payload: resultPayload})
		if err != nil || event.Kind != test.eventKind || event.TransactionID != "tx-1" {
			t.Fatalf("decoded = %+v, error = %v", event, err)
		}
	}
}

func TestKafkaCodecRejectsMalformedContract(t *testing.T) {
	codec := NewKafka(codecTestConfig())
	_, err := codec.Decode(&platformkafka.Message{EventType: messagebus.PaymentCompletedType, Payload: []byte(`{"transaction_id":""}`)})
	if !errors.Is(err, processcore.ErrInvalidEvent) {
		t.Fatalf("error = %v", err)
	}
}

func codecTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Kafka.Topics = config.KafkaTopics{
		PaymentEvents: "payments.events", AffiliateCommands: "affiliate.commands", AffiliateEvents: "affiliate.events",
		SettlementCommands: "settlement.commands", SettlementEvents: "settlement.events",
		AgentCommands: "agent.commands", AgentEvents: "agent.events",
	}
	return cfg
}
