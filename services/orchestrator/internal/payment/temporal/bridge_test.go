package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"
)

type gatewayStub struct {
	delivered []payment.Event
	err       error
}

func (g *gatewayStub) DeliverPayment(_ context.Context, event payment.Event) error {
	g.delivered = append(g.delivered, event)
	return g.err
}

func TestBridgeStartsAndSignalsPaymentWorkflow(t *testing.T) {
	gateway := &gatewayStub{}
	cfg := temporalTestConfig()
	bridge := NewBridge(gateway, codec.NewKafka(cfg))
	if err := bridge.Handle(context.Background(), bridgeMessage("payment", messagebus.PaymentCompletedType, messagebus.PaymentCompleted{
		TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	})); err != nil {
		t.Fatal(err)
	}
	if len(gateway.delivered) != 1 || gateway.delivered[0].TransactionID != "tx-1" {
		t.Fatalf("delivered = %+v", gateway.delivered)
	}
	if gateway.delivered[0].OrchestrationOwner != "temporal:v1" || gateway.delivered[0].OrchestrationGeneration != 1 {
		t.Fatalf("orchestration identity = %q generation %d", gateway.delivered[0].OrchestrationOwner, gateway.delivered[0].OrchestrationGeneration)
	}

	events := []struct {
		typeName string
		payload  any
		kind     payment.EventKind
	}{
		{messagebus.VolumeAccumulatedType, messagebus.VolumeAccumulated{TransactionID: "tx-1", AgentID: "agent-1", CurrentRank: "SILVER"}, payment.EventVolumeAccumulated},
		{messagebus.CommissionCreditedType, messagebus.CommissionCredited{TransactionID: "tx-1", AgentID: "agent-1"}, payment.EventCommissionCredited},
		{messagebus.CallbackDispatchSkippedType, messagebus.CallbackResult{TransactionID: "tx-1", AgentID: "agent-1", Status: "SKIPPED"}, payment.EventCallbackCompleted},
	}
	for _, event := range events {
		if err := bridge.Handle(context.Background(), bridgeMessage(event.typeName, event.typeName, event.payload)); err != nil {
			t.Fatalf("Handle(%s) error = %v", event.typeName, err)
		}
		if got := gateway.delivered[len(gateway.delivered)-1]; got.TransactionID != "tx-1" || got.Kind != event.kind {
			t.Fatalf("signal = %+v", got)
		}
	}
}

func TestBridgeRejectsMalformedEventsAndRetriesTemporalOutage(t *testing.T) {
	gateway := &gatewayStub{}
	bridge := NewBridge(gateway, codec.NewKafka(temporalTestConfig()))
	err := bridge.Handle(context.Background(), &kafka.Message{ID: "bad", EventType: messagebus.PaymentCompletedType, Payload: []byte(`{"transaction_id":""}`)})
	if !kafka.IsPermanent(err) {
		t.Fatalf("invalid payload error = %v, want permanent", err)
	}

	gateway.err = errors.New("temporal unavailable")
	err = bridge.Handle(context.Background(), bridgeMessage("payment", messagebus.PaymentCompletedType, messagebus.PaymentCompleted{
		TransactionID: "tx-1", AgentID: "agent-1", Amount: 100, Currency: "USD",
	}))
	if err == nil || kafka.IsPermanent(err) {
		t.Fatalf("Temporal outage error = %v, want retryable", err)
	}
}

func bridgeMessage(id, eventType string, payload any) *kafka.Message {
	encoded, _ := json.Marshal(payload)
	return &kafka.Message{ID: id, EventType: eventType, Payload: encoded}
}
