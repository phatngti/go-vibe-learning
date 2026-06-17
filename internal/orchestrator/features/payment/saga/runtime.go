package saga

import (
	"context"
	"time"

	"github/master_go_cqrs/internal/observability/logger"
	sagacore "github/master_go_cqrs/internal/orchestrator/core/saga"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"

	"go.uber.org/fx"
)

type Handler struct {
	core *sagacore.Handler
}

func NewHandler(process *Process, inboxRepo inbox.Repository) *Handler {
	return &Handler{core: sagacore.NewHandler(process.repo.DB(), inboxRepo, process)}
}

func (h *Handler) Handle(ctx context.Context, message *kafka.Message) error {
	return h.core.Handle(ctx, message)
}

type Reconciler struct {
	core *sagacore.Reconciler
}

func NewReconciler(process *Process, l logger.Factory) *Reconciler {
	return &Reconciler{core: sagacore.NewReconciler(process, l, sagacore.ReconcilerConfig{
		Interval: 10 * time.Second, BatchSize: 50,
	})}
}

func (r *Reconciler) Start(lifecycle fx.Lifecycle) { r.core.Start(lifecycle) }

func (r *Reconciler) Reconcile(ctx context.Context) { r.core.Reconcile(ctx) }
