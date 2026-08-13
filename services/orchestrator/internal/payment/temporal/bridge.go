package temporal

import (
	"context"
	"errors"

	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/config"
	processcore "github/master_go_cqrs/services/orchestrator/internal/core/process"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"
)

type Bridge struct {
	gateway WorkflowGateway
	codec   *codec.Kafka
}

func NewBridge(gateway WorkflowGateway, kafkaCodec *codec.Kafka) *Bridge {
	return &Bridge{gateway: gateway, codec: kafkaCodec}
}

func (b *Bridge) Handle(ctx context.Context, msg *kafka.Message) error {
	event, err := b.codec.Decode(msg)
	if err != nil {
		if errors.Is(err, processcore.ErrInvalidEvent) {
			return kafka.Permanent(err)
		}
		return err
	}
	if event.Kind == payment.EventPaymentCompleted {
		return b.gateway.StartPayment(ctx, event)
	}
	return b.gateway.SignalPayment(ctx, event)
}

func NewPaymentSubscription(bridge *Bridge, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.PaymentEvents, GroupID: "orchestrator-payments", Handler: bridge.Handle}
}

func NewAffiliateSubscription(bridge *Bridge, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AffiliateEvents, GroupID: "orchestrator-affiliate-events", Handler: bridge.Handle}
}

func NewSettlementSubscription(bridge *Bridge, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.SettlementEvents, GroupID: "orchestrator-settlement-events", Handler: bridge.Handle}
}

func NewAgentSubscription(bridge *Bridge, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AgentEvents, GroupID: "orchestrator-agent-events", Handler: bridge.Handle}
}
