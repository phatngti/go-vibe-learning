package temporal

import (
	"context"
	"encoding/json"
	"testing"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/orchestrator/features/payment"
	"github/master_go_cqrs/internal/orchestrator/features/payment/codec"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
)

type publishedMessage struct {
	topic string
	key   string
	msg   kafka.Message
}

type producerStub struct {
	published []publishedMessage
	err       error
}

func (p *producerStub) Publish(_ context.Context, topic, key string, msg *kafka.Message) error {
	p.published = append(p.published, publishedMessage{topic: topic, key: key, msg: *msg})
	return p.err
}

func (*producerStub) Close() error { return nil }

func TestCommandActivityPublishesExistingKafkaContracts(t *testing.T) {
	cfg := temporalTestConfig()
	producer := &producerStub{}
	activity := NewCommandActivity(producer, codec.NewKafka(cfg))
	tests := []struct {
		kind        payment.CommandKind
		topic       string
		messageType string
		step        string
	}{
		{payment.CommandAccumulateVolume, cfg.Kafka.Topics.AffiliateCommands, messagebus.AccumulateVolumeRequestedType, messagebus.StepAffiliate},
		{payment.CommandCreditCommission, cfg.Kafka.Topics.SettlementCommands, messagebus.CreditCommissionRequestedType, messagebus.StepSettlement},
		{payment.CommandDispatchCallback, cfg.Kafka.Topics.AgentCommands, messagebus.DispatchCallbackRequestedType, messagebus.StepCallback},
	}
	for _, test := range tests {
		if err := activity.Publish(context.Background(), payment.Command{
			Kind: test.kind, TransactionID: "tx-1", AgentID: "agent-1",
			Amount: 100, Currency: "USD", Rank: "SILVER",
		}); err != nil {
			t.Fatalf("Publish(%s) error = %v", test.kind, err)
		}
		published := producer.published[len(producer.published)-1]
		if published.topic != test.topic || published.key != "tx-1" || published.msg.EventType != test.messageType {
			t.Fatalf("unexpected publish: %+v", published)
		}
		if published.msg.ID != messagebus.MessageID("tx-1", test.step) {
			t.Fatalf("message id = %q", published.msg.ID)
		}
		if !json.Valid(published.msg.Payload) {
			t.Fatalf("payload is not JSON: %q", published.msg.Payload)
		}
	}
}

func temporalTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Kafka.Topics = config.KafkaTopics{
		PaymentEvents: "payments.events", AffiliateCommands: "affiliate.commands", AffiliateEvents: "affiliate.events",
		SettlementCommands: "settlement.commands", SettlementEvents: "settlement.events",
		AgentCommands: "agent.commands", AgentEvents: "agent.events",
	}
	cfg.Orchestrator.Engine = config.OrchestratorEngineTemporal
	cfg.Orchestrator.Temporal.Address = "temporal:7233"
	cfg.Orchestrator.Temporal.Namespace = "master-go-cqrs"
	cfg.Orchestrator.Temporal.TaskQueue = "payment-orchestration"
	return cfg
}
