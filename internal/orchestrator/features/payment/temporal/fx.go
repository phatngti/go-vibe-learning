package temporal

import (
	"context"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"
	temporalcore "github/master_go_cqrs/internal/orchestrator/core/temporal"
	"github/master_go_cqrs/internal/orchestrator/features/payment/codec"
	"github/master_go_cqrs/internal/platform/kafka"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("orchestrator-temporal",
	fx.Provide(
		temporalcore.NewClient,
		codec.NewKafka,
		NewGateway,
		NewCommandActivity,
		NewBridge,
	),
	kafka.AsSubscription(NewPaymentSubscription),
	kafka.AsSubscription(NewAffiliateSubscription),
	kafka.AsSubscription(NewSettlementSubscription),
	kafka.AsSubscription(NewAgentSubscription),
	fx.Invoke(RegisterWorker),
)

func RegisterWorker(lifecycle fx.Lifecycle, c client.Client, commands *CommandActivity, cfg *config.Config, l logger.Factory) {
	w := worker.New(c, cfg.Orchestrator.Temporal.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(PaymentWorkflow, workflow.RegisterOptions{Name: PaymentWorkflowName})
	w.RegisterActivityWithOptions(commands.Publish, activity.RegisterOptions{Name: PublishCommandActivityName})
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			l.Bg().Info("Starting Temporal orchestrator worker", zap.String("task_queue", cfg.Orchestrator.Temporal.TaskQueue))
			return w.Start()
		},
		OnStop: func(context.Context) error {
			l.Bg().Info("Stopping Temporal orchestrator worker")
			w.Stop()
			return nil
		},
	})
}
