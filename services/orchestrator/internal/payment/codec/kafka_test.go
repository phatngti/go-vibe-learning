package codec

import (
	"encoding/json"
	"errors"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	platformkafka "github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/config"
	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
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
			command:     payment.Command{Kind: payment.CommandCreditCommission, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Rank: "SILVER"},
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
		if encoded.Message.OrchestrationOwner != "saga:v1" || encoded.Message.OrchestrationGeneration != 1 {
			t.Fatalf("encoded orchestration identity = %+v", encoded.Message)
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

func TestKafkaCodecIncludesGenerationInNewCommandIdentity(t *testing.T) {
	cfg := codecTestConfig()
	cfg.Orchestrator.Engine = config.OrchestratorEngineTemporal
	cfg.Orchestrator.Generation = 2
	codec := NewKafka(cfg)
	encoded, err := codec.Encode(payment.Command{
		Kind: payment.CommandAccumulateVolume, TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if encoded.Message.ID != "tx-1:affiliate:g2" {
		t.Fatalf("command ID = %q", encoded.Message.ID)
	}
	if encoded.Message.OrchestrationOwner != "temporal:v2" || encoded.Message.OrchestrationGeneration != 2 {
		t.Fatalf("command orchestration identity = %+v", encoded.Message)
	}
}

func TestKafkaCodecPropagatesAuthoritativeMoneyV2(t *testing.T) {
	codec := NewKafka(codecTestConfig())
	authoritative := messagebus.Money{AmountMinor: 10_001, Currency: "USD", Exponent: 2}
	commands := []payment.Command{
		{Kind: payment.CommandAccumulateVolume, TransactionID: "tx-money", AgentID: "agent-1", Money: &authoritative, Amount: 999.99, Currency: "USD"},
		{Kind: payment.CommandCreditCommission, TransactionID: "tx-money", AgentID: "agent-1", Money: &authoritative, Amount: 999.99, Currency: "USD", Rank: "SILVER"},
		{Kind: payment.CommandDispatchCallback, TransactionID: "tx-money", AgentID: "agent-1", Money: &authoritative, Amount: 999.99, Currency: "USD"},
	}
	for _, command := range commands {
		encoded, err := codec.Encode(command)
		if err != nil {
			t.Fatal(err)
		}
		var contract struct {
			Money             *messagebus.Money `json:"money"`
			TransactionMoney  *messagebus.Money `json:"transaction_money"`
			Amount            float64           `json:"amount"`
			TransactionAmount float64           `json:"transaction_amount"`
		}
		if err := json.Unmarshal(encoded.Message.Payload, &contract); err != nil {
			t.Fatal(err)
		}
		propagated := contract.Money
		legacy := contract.Amount
		if propagated == nil {
			propagated = contract.TransactionMoney
			legacy = contract.TransactionAmount
		}
		if propagated == nil || *propagated != authoritative || legacy != 100.01 {
			t.Fatalf("%s payload = %s", command.Kind, encoded.Message.Payload)
		}
	}

	paymentPayload, _ := json.Marshal(messagebus.PaymentCompleted{
		TransactionID: "tx-money", AgentID: "agent-1", Money: &authoritative,
		Amount: 999.99, Currency: "USD",
	})
	event, err := codec.Decode(&platformkafka.Message{EventType: messagebus.PaymentCompletedType, Payload: paymentPayload})
	if err != nil {
		t.Fatal(err)
	}
	if event.Money == nil || *event.Money != authoritative {
		t.Fatalf("decoded event money = %+v", event.Money)
	}
}

func TestKafkaCodecRejectsMalformedContract(t *testing.T) {
	codec := NewKafka(codecTestConfig())
	_, err := codec.Decode(&platformkafka.Message{EventType: messagebus.PaymentCompletedType, Payload: []byte(`{"transaction_id":""}`)})
	if !errors.Is(err, processcore.ErrInvalidEvent) {
		t.Fatalf("error = %v", err)
	}
}

func TestKafkaCodecDecodesDefinitiveAndUncertainPaymentOutcomes(t *testing.T) {
	codec := NewKafka(codecTestConfig())
	tests := []struct {
		typeName string
		payload  any
		want     payment.EventKind
	}{
		{messagebus.PaymentFailedType, messagebus.PaymentFailed{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Reason: "declined"}, payment.EventPaymentFailed},
		{messagebus.PaymentProcessingUncertainType, messagebus.PaymentProcessingUncertain{TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD", Reason: "timeout"}, payment.EventPaymentUncertain},
	}
	for _, test := range tests {
		payload, _ := json.Marshal(test.payload)
		event, err := codec.Decode(&platformkafka.Message{ID: "message-1", EventType: test.typeName, Payload: payload})
		if err != nil || event.Kind != test.want || event.MessageID != "message-1" {
			t.Fatalf("event = %+v, error = %v", event, err)
		}
	}
}

func codecTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Orchestrator.Engine = config.OrchestratorEngineSaga
	cfg.Orchestrator.Generation = 1
	cfg.Kafka.Topics = config.KafkaTopics{
		PaymentEvents: "payments.events", AffiliateCommands: "affiliate.commands", AffiliateEvents: "affiliate.events",
		SettlementCommands: "settlement.commands", SettlementEvents: "settlement.events",
		AgentCommands: "agent.commands", AgentEvents: "agent.events",
	}
	return cfg
}
