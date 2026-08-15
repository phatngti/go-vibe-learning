// File: internal/platform/kafka/fx.go
// Description: Fx dependency injection modules for Kafka producers, consumers, and subscriptions.

package kafka

import (
	"context"

	"go.uber.org/fx"
)

var ProducerModule = fx.Module("kafka-producer",
	fx.Provide(NewProducer),
	fx.Invoke(func(lifecycle fx.Lifecycle, producer Producer) {
		lifecycle.Append(fx.Hook{OnStop: func(context.Context) error { return producer.Close() }})
	}),
)

var ConsumerModule = fx.Module("kafka-consumer",
	fx.Provide(NewConsumer),
	fx.Invoke(RunSubscriptions),
)

var Module = fx.Module("kafka", ProducerModule, ConsumerModule)
