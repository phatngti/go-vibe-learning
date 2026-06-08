package main

import (
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/security/gatewayauth"
	"github/master_go_cqrs/internal/settlement"

	"go.uber.org/fx"
)

const timeout = 30 * time.Second

func main() {
	fx.New(
		config.Module,
		logger.Module,
		telemetry.Module,
		postgres.Module,
		gatewayauth.Module,
		settlement.Module,
		httpapi.Module,
		fx.Provide(grpc.NewServer, grpc.ProvideRawServer),
		fx.StartTimeout(timeout),
		fx.StopTimeout(timeout),
	).Run()
}
