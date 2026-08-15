package saga

import (
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/config"
)

func NewPaymentInitiatedSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.PaymentInitiatedEvents, GroupID: cfg.ConsumerGroupID("payments-initiated"), Handler: handler.Handle}
}

func NewPaymentSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.PaymentEvents, GroupID: cfg.ConsumerGroupID("payments"), Handler: handler.Handle}
}

func NewAffiliateSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AffiliateEvents, GroupID: cfg.ConsumerGroupID("affiliate-events"), Handler: handler.Handle}
}

func NewSettlementSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.SettlementEvents, GroupID: cfg.ConsumerGroupID("settlement-events"), Handler: handler.Handle}
}

func NewAgentSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AgentEvents, GroupID: cfg.ConsumerGroupID("agent-events"), Handler: handler.Handle}
}
