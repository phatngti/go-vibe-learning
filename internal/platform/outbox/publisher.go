// File: internal/platform/outbox/publisher.go
// Description: Outbox Publisher daemon running background batch loops to publish claimed outbox events to Kafka.

package outbox

import (
	"context"
	"sync"
	"time"

	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Publisher struct {
	repo     Repository
	producer kafka.Producer
	logger   logger.Factory
	workerID string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewPublisher(repo Repository, producer kafka.Producer, l logger.Factory) *Publisher {
	return &Publisher{repo: repo, producer: producer, logger: l, workerID: uuid.NewString()}
}

func (p *Publisher) Start(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			p.cancel = cancel
			p.wg.Add(1)
			go p.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			if p.cancel != nil {
				p.cancel()
			}
			p.wg.Wait()
			return nil
		},
	})
}

func (p *Publisher) run(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := p.publishBatch(ctx); err != nil && ctx.Err() == nil {
			p.logger.Bg().Error("Outbox publish batch failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.repo.Claim(ctx, p.workerID, 20, time.Minute)
	if err != nil {
		return err
	}
	for _, event := range events {
		message := &kafka.Message{
			ID: event.MessageID, EventType: event.MessageType,
			AggregateType: event.AggregateType, AggregateID: event.AggregateID,
			Payload: []byte(event.Payload), OccurredAt: event.CreatedAt,
		}
		if err := p.producer.Publish(ctx, event.Topic, event.MessageKey, message); err != nil {
			delay := time.Duration(1<<min(event.Attempts, 5)) * time.Second
			_ = p.repo.MarkRetry(ctx, event.ID, err.Error(), delay)
			continue
		}
		if err := p.repo.MarkProcessed(ctx, event.ID); err != nil {
			return err
		}
	}
	return nil
}
