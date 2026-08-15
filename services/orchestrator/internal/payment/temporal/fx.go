package temporal

import (
	"context"

	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/services/orchestrator/internal/config"
	temporalcore "github/master_go_cqrs/services/orchestrator/internal/core/temporal"
	"github/master_go_cqrs/services/orchestrator/internal/payment/codec"

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
	kafka.AsSubscription(NewPaymentInitiatedSubscription),
	kafka.AsSubscription(NewPaymentSubscription),
	kafka.AsSubscription(NewAffiliateSubscription),
	kafka.AsSubscription(NewSettlementSubscription),
	kafka.AsSubscription(NewAgentSubscription),
	fx.Invoke(RegisterWorker),
)

func RegisterWorker(lifecycle fx.Lifecycle, c client.Client, commands *CommandActivity, cfg *config.Config, l logger.Factory) {
	settings := cfg.Orchestrator.Temporal
	workerOptions := worker.Options{}
	if settings.VersioningEnabled {
		workerOptions.DeploymentOptions = worker.DeploymentOptions{
			UseVersioning: true,
			Version: worker.WorkerDeploymentVersion{
				DeploymentName: settings.DeploymentName,
				BuildID:        settings.BuildID,
			},
			DefaultVersioningBehavior: workflow.VersioningBehaviorPinned,
		}
	}
	w := worker.New(c, settings.TaskQueue, workerOptions)
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
