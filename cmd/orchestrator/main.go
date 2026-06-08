package main

import (
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/orchestrator"
	"github/master_go_cqrs/internal/platform/temporal"

	"go.uber.org/fx"
)

const timeout = 30 * time.Second

func main() {
	fx.New(
		config.Module,
		logger.Module,
		telemetry.Module,
		temporal.Module,
		orchestrator.Module,
		fx.StartTimeout(timeout),
		fx.StopTimeout(timeout),
	).Run()
}
