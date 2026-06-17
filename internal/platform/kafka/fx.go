// File: internal/platform/kafka/fx.go
// Description: Fx dependency injection modules for Kafka producers, consumers, and subscriptions.

package kafka

import (
	"context"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"go.uber.org/fx"
)

var ProducerModule = fx.Module("kafka-producer",
	fx.Provide(func(cfg *config.Config) Producer { return NewProducer(cfg.Kafka.Brokers) }),
	fx.Invoke(func(lifecycle fx.Lifecycle, producer Producer) {
		lifecycle.Append(fx.Hook{OnStop: func(context.Context) error { return producer.Close() }})
	}),
)

var ConsumerModule = fx.Module("kafka-consumer",
	fx.Provide(func(cfg *config.Config, producer Producer, l logger.Factory) Consumer {
		return NewConsumer(cfg.Kafka.Brokers, producer, l)
	}),
	fx.Invoke(RunSubscriptions),
)

var Module = fx.Module("kafka", ProducerModule, ConsumerModule)
