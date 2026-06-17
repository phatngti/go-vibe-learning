package saga

import (
	"context"
	"sync"
	"time"

	"github/master_go_cqrs/internal/observability/logger"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// RetryProcessor defines the feature-specific retry policy for a long-running process.
type RetryProcessor interface {
	Name() string
	Due(ctx context.Context, limit int) ([]string, error)
	Retry(ctx context.Context, instanceID string, now time.Time) error
}

type ReconcilerConfig struct {
	Interval  time.Duration
	BatchSize int
}

type Reconciler struct {
	processor RetryProcessor
	logger    logger.Factory
	config    ReconcilerConfig
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewReconciler(processor RetryProcessor, l logger.Factory, config ReconcilerConfig) *Reconciler {
	if config.Interval <= 0 {
		config.Interval = 10 * time.Second
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	return &Reconciler{processor: processor, logger: l, config: config}
}

func (r *Reconciler) Start(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			r.cancel = cancel
			r.wg.Add(1)
			go r.run(ctx)
			return nil
		},
		OnStop: func(context.Context) error {
			if r.cancel != nil {
				r.cancel()
			}
			r.wg.Wait()
			return nil
		},
	})
}

func (r *Reconciler) run(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Reconcile(ctx)
		}
	}
}

func (r *Reconciler) Reconcile(ctx context.Context) {
	instanceIDs, err := r.processor.Due(ctx, r.config.BatchSize)
	if err != nil {
		r.logger.Bg().Error("Saga reconciliation query failed",
			zap.String("process", r.processor.Name()), zap.Error(err))
		return
	}
	for _, instanceID := range instanceIDs {
		if err := r.processor.Retry(ctx, instanceID, time.Now()); err != nil {
			r.logger.Bg().Error("Saga reconciliation failed",
				zap.String("process", r.processor.Name()),
				zap.String("instance_id", instanceID), zap.Error(err))
		}
	}
}
