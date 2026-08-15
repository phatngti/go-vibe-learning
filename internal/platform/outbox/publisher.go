// File: internal/platform/outbox/publisher.go
// Description: Horizontally safe ordered outbox publisher with bounded retries.

package outbox

import (
	"context"
	"sync"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messageidentity"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	outboxMeter               = otel.Meter("github/master_go_cqrs/internal/platform/outbox")
	outboxClaimed, _          = outboxMeter.Int64Counter("outbox.events.claimed")
	outboxPublished, _        = outboxMeter.Int64Counter("outbox.events.published")
	outboxPublishFailures, _  = outboxMeter.Int64Counter("outbox.publish.failures")
	outboxDead, _             = outboxMeter.Int64Counter("outbox.events.dead")
	outboxPublishDuration, _  = outboxMeter.Float64Histogram("outbox.publish.duration", metric.WithUnit("s"))
	outboxQueueDepth, _       = outboxMeter.Int64Gauge("outbox.queue.depth")
	outboxOldestPendingAge, _ = outboxMeter.Float64Gauge("outbox.pending.oldest_age", metric.WithUnit("s"))
)

type Publisher struct {
	repo     Repository
	producer kafka.Producer
	logger   logger.Factory
	config   platformconfig.Outbox
	workerID string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewPublisher(repo Repository, producer kafka.Producer, l logger.Factory, cfg *platformconfig.Outbox) *Publisher {
	settings := platformconfig.Outbox{Enabled: true}
	if cfg != nil {
		settings = *cfg
	}
	if settings.PollInterval <= 0 {
		settings.PollInterval = 100 * time.Millisecond
	}
	if settings.MetricsInterval <= 0 {
		settings.MetricsInterval = 15 * time.Second
	}
	if settings.BatchSize <= 0 {
		settings.BatchSize = 500
	}
	if settings.Parallelism <= 0 {
		settings.Parallelism = 16
	}
	if settings.StaleAfter <= 0 {
		settings.StaleAfter = 2 * time.Minute
	}
	if settings.MaxAttempts <= 0 {
		settings.MaxAttempts = 20
	}
	return &Publisher{repo: repo, producer: producer, logger: l, config: settings, workerID: uuid.NewString()}
}

func (p *Publisher) Start(lifecycle fx.Lifecycle) {
	if !p.config.Enabled {
		p.logger.Bg().Info("Outbox polling publisher disabled; external CDC relay is expected")
		return
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			p.cancel = cancel
			p.wg.Add(2)
			go p.run(ctx)
			go p.observe(ctx)
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
	for {
		published, err := p.publishBatch(ctx)
		if err != nil && ctx.Err() == nil {
			p.logger.Bg().Error("Outbox publish batch failed", zap.Error(err))
		}
		// Drain a full batch immediately. This removes the old fixed 20 msg/s
		// ceiling while still backing off when the queue is empty or degraded.
		if err == nil && published == p.config.BatchSize {
			continue
		}
		timer := time.NewTimer(p.config.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) (int, error) {
	events, err := p.repo.Claim(ctx, p.workerID, p.config.BatchSize, p.config.StaleAfter)
	if err != nil {
		return 0, err
	}
	outboxClaimed.Add(ctx, int64(len(events)))
	sem := make(chan struct{}, p.config.Parallelism)
	var wg sync.WaitGroup
	for _, event := range events {
		event := event
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			aggregateVersion := event.AggregateVersion
			if aggregateVersion == 0 {
				if version, ok := messageidentity.FactVersion(event.MessageID); ok {
					aggregateVersion = version
				}
			}
			message := &kafka.Message{
				ID: event.MessageID, EventType: event.MessageType,
				AggregateType: event.AggregateType, AggregateID: event.AggregateID,
				Payload: []byte(event.Payload), OccurredAt: event.CreatedAt,
				SchemaVersion: event.SchemaVersion, AggregateVersion: aggregateVersion,
				TenantID: event.TenantID, CorrelationID: event.CorrelationID, CausationID: event.CausationID,
				OrchestrationOwner: event.OrchestrationOwner, OrchestrationGeneration: event.OrchestrationGeneration,
				TraceParent: event.TraceParent, TraceState: event.TraceState,
			}
			startedAt := time.Now()
			topicAttr := metric.WithAttributes(attribute.String("topic", event.Topic))
			if err := p.producer.Publish(ctx, event.Topic, event.MessageKey, message); err != nil {
				outboxPublishDuration.Record(ctx, time.Since(startedAt).Seconds(), topicAttr)
				outboxPublishFailures.Add(ctx, 1, topicAttr)
				delay := time.Duration(1<<min(event.Attempts, 8)) * time.Second
				if retryErr := p.repo.MarkRetry(ctx, event.ID, p.workerID, err.Error(), delay, p.config.MaxAttempts); retryErr != nil {
					p.logger.Bg().Error("Outbox retry update failed", zap.String("message_id", event.MessageID), zap.Error(retryErr))
				} else if event.Attempts+1 >= p.config.MaxAttempts {
					outboxDead.Add(ctx, 1, topicAttr)
				}
				return
			}
			outboxPublishDuration.Record(ctx, time.Since(startedAt).Seconds(), topicAttr)
			if err := p.repo.MarkProcessed(ctx, event.ID, p.workerID); err != nil {
				p.logger.Bg().Error("Outbox processed update failed", zap.String("message_id", event.MessageID), zap.Error(err))
				return
			}
			outboxPublished.Add(ctx, 1, topicAttr)
		}()
	}
	wg.Wait()
	return len(events), nil
}

func (p *Publisher) observe(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.config.MetricsInterval)
	defer ticker.Stop()
	for {
		p.recordStats(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) recordStats(ctx context.Context) {
	stats, err := p.repo.Stats(ctx)
	if err != nil {
		if ctx.Err() == nil {
			p.logger.Bg().Warn("Outbox metrics query failed", zap.Error(err))
		}
		return
	}
	outboxQueueDepth.Record(ctx, stats.PendingCount, metric.WithAttributes(attribute.String("status", StatusPending)))
	outboxQueueDepth.Record(ctx, stats.ProcessingCount, metric.WithAttributes(attribute.String("status", StatusProcessing)))
	outboxQueueDepth.Record(ctx, stats.DeadCount, metric.WithAttributes(attribute.String("status", StatusDead)))
	outboxOldestPendingAge.Record(ctx, stats.OldestPendingAge.Seconds())
}
