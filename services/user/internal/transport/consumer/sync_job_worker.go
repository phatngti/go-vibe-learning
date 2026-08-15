package consumer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/services/user/internal/application/command"
	userconfig "github/master_go_cqrs/services/user/internal/config"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// UserSyncJobWorker horizontally processes durable jobs. Each lane has a
// distinct lease owner and performs only one page before releasing the job,
// allowing fair scheduling and restart-safe progress.
type UserSyncJobWorker struct {
	runner      *command.UserSyncJobRunner
	logger      logger.Factory
	enabled     bool
	parallelism int
	poll        time.Duration
	workerBase  string
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewUserSyncJobWorker(
	runner *command.UserSyncJobRunner,
	cfg *userconfig.Config,
	logger logger.Factory,
) *UserSyncJobWorker {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "user-service"
	}
	return &UserSyncJobWorker{
		runner: runner, logger: logger, enabled: cfg.SyncJobs.Enabled,
		parallelism: cfg.SyncJobs.Parallelism, poll: cfg.SyncJobs.PollInterval,
		workerBase: hostname + ":" + uuid.NewString(),
	}
}

func (w *UserSyncJobWorker) Start(lifecycle fx.Lifecycle) {
	if !w.enabled {
		w.logger.Bg().Info("Durable user synchronization worker disabled")
		return
	}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			w.cancel = cancel
			for lane := 0; lane < w.parallelism; lane++ {
				w.wg.Add(1)
				go w.run(ctx, fmt.Sprintf("%s:%d", w.workerBase, lane))
			}
			return nil
		},
		OnStop: func(context.Context) error {
			if w.cancel != nil {
				w.cancel()
			}
			w.wg.Wait()
			return nil
		},
	})
}

func (w *UserSyncJobWorker) run(ctx context.Context, workerID string) {
	defer w.wg.Done()
	for {
		claimed, err := w.runner.RunNext(ctx, workerID)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.logger.For(ctx).Error("Durable user synchronization step failed").WithFields(
				zap.String("worker_id", workerID), zap.Error(err),
			)
		}
		if claimed && err == nil {
			continue
		}
		delay := w.poll
		if claimed && delay > 100*time.Millisecond {
			delay = 100 * time.Millisecond
		}
		if !waitForSyncJobPoll(ctx, delay) {
			return
		}
	}
}

func waitForSyncJobPoll(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
