package saga

import (
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/platform/kafka"
)

func NewPaymentInitiatedSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.PaymentInitiatedEvents, GroupID: "orchestrator-payments-initiated", Handler: handler.Handle}
}

func NewPaymentSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.PaymentEvents, GroupID: "orchestrator-payments", Handler: handler.Handle}
}

func NewAffiliateSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AffiliateEvents, GroupID: "orchestrator-affiliate-events", Handler: handler.Handle}
}

func NewSettlementSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.SettlementEvents, GroupID: "orchestrator-settlement-events", Handler: handler.Handle}
}

func NewAgentSubscription(handler *Handler, cfg *config.Config) kafka.Subscription {
	return kafka.Subscription{Topic: cfg.Kafka.Topics.AgentEvents, GroupID: "orchestrator-agent-events", Handler: handler.Handle}
}
