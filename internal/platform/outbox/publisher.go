package outbox

import (
	"context"
	"time"

	"github/master_go_cqrs/internal/observability/logger"

	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Publisher struct {
	repo           Repository
	temporalClient client.Client
	logger         logger.Factory
	stopChan       chan struct{}
}

func NewPublisher(repo Repository, temporalClient client.Client, l logger.Factory) *Publisher {
	return &Publisher{
		repo:           repo,
		temporalClient: temporalClient,
		logger:         l,
		stopChan:       make(chan struct{}),
	}
}

func (p *Publisher) Start(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.logger.Bg().Info("Starting Outbox Event Publisher background worker")
			go p.run()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.logger.Bg().Info("Stopping Outbox Event Publisher background worker")
			close(p.stopChan)
			return nil
		},
	})
}

func (p *Publisher) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.publishPendingEvents()
		case <-p.stopChan:
			return
		}
	}
}

func (p *Publisher) publishPendingEvents() {
	ctx := context.Background()
	events, err := p.repo.GetPendingEvents(ctx, 20)
	if err != nil {
		p.logger.Bg().Error("Failed to fetch pending outbox events", zap.Error(err))
		return
	}

	for _, event := range events {
		p.logger.Bg().Info("Publishing outbox event",
			zap.String("id", event.ID.String()),
			zap.String("type", event.EventType),
		)

		var publishErr error
		switch event.EventType {
		case "PaymentCompletedEvent":
			// Trigger Temporal Workflow: PaymentProcessingWorkflow
			options := client.StartWorkflowOptions{
				ID:        "payment-processing-flow-" + event.AggregateID,
				TaskQueue: "payment-processing",
			}
			// Trigger the workflow using the Temporal client
			_, publishErr = p.temporalClient.ExecuteWorkflow(
				ctx,
				options,
				"PaymentProcessingWorkflow",
				event.Payload, // Pass payload directly
			)
		default:
			p.logger.Bg().Warn("Unknown outbox event type; marking processed", zap.String("type", event.EventType))
		}

		if publishErr != nil {
			p.logger.Bg().Error("Failed to publish outbox event, retrying later", zap.String("id", event.ID.String()), zap.Error(publishErr))
			_ = p.repo.MarkAsFailed(ctx, event.ID, publishErr.Error())
		} else {
			p.logger.Bg().Info("Successfully published outbox event", zap.String("id", event.ID.String()))
			_ = p.repo.MarkAsProcessed(ctx, event.ID)
		}
	}
}
