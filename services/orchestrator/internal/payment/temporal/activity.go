package temporal

import (
	"context"

	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"

	temporalsdk "go.temporal.io/sdk/temporal"
)

type CommandActivity struct {
	producer kafka.Producer
	codec    *codec.Kafka
}

func NewCommandActivity(producer kafka.Producer, kafkaCodec *codec.Kafka) *CommandActivity {
	return &CommandActivity{producer: producer, codec: kafkaCodec}
}

func (a *CommandActivity) Publish(ctx context.Context, command payment.Command) error {
	encoded, err := a.codec.Encode(command)
	if err != nil {
		return temporalsdk.NewNonRetryableApplicationError("encode payment saga command", "InvalidCommand", err)
	}
	return a.producer.Publish(ctx, encoded.Topic, encoded.Key, encoded.Message)
}
