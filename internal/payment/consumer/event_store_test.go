package consumer

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/payment/domain"
	"github/master_go_cqrs/internal/platform/eventstore"

	"github.com/google/uuid"
)

func TestEventStoreHandlerDecodesPaymentEvents(t *testing.T) {
	cfg := &config.Config{}
	cfg.Logger.LogLevel = "error"
	cfg.Logger.Mode = "production"
	cfg.Logger.LogEncoding = "json"
	handler := NewEventStoreHandler(logger.NewFactory(logger.NewapiLogger(cfg)))

	tests := []struct {
		eventType string
		payload   string
	}{
		{(domain.PaymentInitiated{}).EventType(), `{"transaction_id":"tx-1","agent_id":"agent-1","amount":100,"currency":"USD"}`},
		{(domain.PaymentCompleted{}).EventType(), `{"transaction_id":"tx-1","agent_id":"agent-1","amount":100,"currency":"USD"}`},
		{(domain.PaymentFailed{}).EventType(), `{"transaction_id":"tx-1","agent_id":"agent-1","amount":100,"currency":"USD","reason":"declined"}`},
	}
	for _, test := range tests {
		t.Run(test.eventType, func(t *testing.T) {
			err := handler.Handle(context.Background(), nil, &eventstore.DomainEvent{
				ID: uuid.New(), AggregateID: "tx-1", EventType: test.eventType, Payload: test.payload,
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEventStoreHandlerRejectsMalformedEvent(t *testing.T) {
	handler := &EventStoreHandler{}
	err := handler.Handle(context.Background(), nil, &eventstore.DomainEvent{
		ID: uuid.New(), EventType: (domain.PaymentInitiated{}).EventType(), Payload: `{`,
	})
	if err == nil {
		t.Fatal("Handle() error = nil")
	}
}
