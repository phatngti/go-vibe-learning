package orchestrator

import (
	"context"

	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/orchestrator/activities"
	"github/master_go_cqrs/internal/orchestrator/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("orchestrator",
	fx.Provide(
		activities.NewActivities,
	),
	fx.Invoke(RegisterWorker),
)

func RegisterWorker(lifecycle fx.Lifecycle, c client.Client, act *activities.Activities, l logger.Factory) {
	// Worker polls "payment-processing" queue
	w := worker.New(c, "payment-processing", worker.Options{})

	w.RegisterWorkflow(workflows.PaymentProcessingWorkflow)
	w.RegisterActivity(act)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			l.Bg().Info("Starting Temporal Orchestrator worker")
			go func() {
				if err := w.Run(worker.InterruptCh()); err != nil {
					l.Bg().Error("Temporal Worker failed to run", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Bg().Info("Stopping Temporal Orchestrator worker")
			w.Stop()
			return nil
		},
	})
}
