// File: internal/platform/kafka/subscription.go
// Description: Defines Subscription registration types and Fx hooks for starting background consumer loops.

package kafka

import (
	"context"
	"sync"
	"time"

	"github/master_go_cqrs/internal/platform/observability/logger"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Subscription struct {
	Topic   string
	GroupID string
	Handler Handler
}

func AsSubscription(constructor any) fx.Option {
	return fx.Provide(fx.Annotate(constructor, fx.ResultTags(`group:"kafka_subscriptions"`)))
}

type subscriptionParams struct {
	fx.In
	Lifecycle     fx.Lifecycle
	Consumer      Consumer
	Logger        logger.Factory
	Subscriptions []Subscription `group:"kafka_subscriptions"`
}

func RunSubscriptions(params subscriptionParams) {
	var cancel context.CancelFunc
	var wg sync.WaitGroup
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, stop := context.WithCancel(context.Background())
			cancel = stop
			for _, subscription := range params.Subscriptions {
				subscription := subscription
				wg.Add(1)
				go func() {
					defer wg.Done()
					runSubscriptionSupervisor(ctx, params.Consumer, subscription, params.Logger)
				}()
			}
			return nil
		},
		OnStop: func(context.Context) error {
			if cancel != nil {
				cancel()
			}
			_ = params.Consumer.Close()
			wg.Wait()
			return nil
		},
	})
}

func runSubscriptionSupervisor(ctx context.Context, consumer Consumer, subscription Subscription, log logger.Factory) {
	restartAttempt := 0
	for {
		err := consumer.Subscribe(ctx, subscription.Topic, subscription.GroupID, subscription.Handler)
		if ctx.Err() != nil {
			return
		}
		restartAttempt++
		delay := min(time.Duration(1<<min(restartAttempt-1, 6))*time.Second, 30*time.Second)
		log.Bg().Error("Kafka subscription stopped; restarting",
			zap.String("topic", subscription.Topic), zap.String("group_id", subscription.GroupID),
			zap.Duration("restart_delay", delay), zap.Error(err))
		if waitForRetry(ctx, delay) != nil {
			return
		}
	}
}
