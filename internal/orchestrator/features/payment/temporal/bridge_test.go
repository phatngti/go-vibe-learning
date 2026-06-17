package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github/master_go_cqrs/internal/orchestrator/features/payment"
	"github/master_go_cqrs/internal/orchestrator/features/payment/codec"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
)

type gatewayStub struct {
	started []payment.Event
	signals []gatewaySignal
	err     error
}

type gatewaySignal struct {
	event payment.Event
}

func (g *gatewayStub) StartPayment(_ context.Context, event payment.Event) error {
	g.started = append(g.started, event)
	return g.err
}

func (g *gatewayStub) SignalPayment(_ context.Context, event payment.Event) error {
	g.signals = append(g.signals, gatewaySignal{event: event})
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
	if len(gateway.started) != 1 || gateway.started[0].TransactionID != "tx-1" {
		t.Fatalf("started = %+v", gateway.started)
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
		if got := gateway.signals[len(gateway.signals)-1]; got.event.TransactionID != "tx-1" || got.event.Kind != event.kind {
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
