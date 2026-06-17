package agent

import (
	"time"

	"github/master_go_cqrs/internal/agent/application/command"
	"github/master_go_cqrs/internal/agent/application/query"
	agentconsumer "github/master_go_cqrs/internal/agent/consumer"
	"github/master_go_cqrs/internal/agent/delivery/grpc"
	"github/master_go_cqrs/internal/agent/delivery/http"
	"github/master_go_cqrs/internal/agent/repository/postgres"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("agent",
	fx.Provide(
		postgres.NewAgentRepository,
		command.NewConfigureCallbackHandler,
		command.NewGenerateAPIKeyHandler,
		command.NewRecordAuditLogHandler,
		query.NewValidateAPIKeyHandler,
		agentconsumer.NewCallbackCommandHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	kafka.AsSubscription(agentconsumer.NewCallbackSubscription),
	fx.Invoke(grpc.RequireServerHandler),
)

var RuntimeModule = fx.Module("agent-runtime",
	config.Module,
	logger.Module,
	telemetry.Module,
	platformpostgres.Module,
	eventstore.Module,
	kafka.ProducerModule,
	kafka.ConsumerModule,
	inbox.Module,
	outbox.Module,
	gatewayauth.Module,
	Module,
	httpapi.Module,
	fx.Provide(platformgrpc.NewServer, platformgrpc.ProvideRawServer),
)

func Options() fx.Option {
	return fx.Options(RuntimeModule, fx.StartTimeout(lifecycleTimeout), fx.StopTimeout(lifecycleTimeout))
}
