package agent

import (
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/httpapi"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/observability/telemetry"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	agentpostgres "github/master_go_cqrs/services/agent/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/agent/internal/application/command"
	"github/master_go_cqrs/services/agent/internal/application/query"
	"github/master_go_cqrs/services/agent/internal/config"
	agentconsumer "github/master_go_cqrs/services/agent/internal/transport/consumer"
	"github/master_go_cqrs/services/agent/internal/transport/grpc"
	"github/master_go_cqrs/services/agent/internal/transport/http"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("agent",
	fx.Provide(
		// Ports → Adapters
		agentpostgres.NewAgentRepository,
		agentpostgres.NewUnitOfWork,

		// Application — Commands
		command.NewConfigureCallbackHandler,
		command.NewGenerateAPIKeyHandler,
		command.NewRecordAuditLogHandler,

		// Application — Queries
		query.NewValidateAPIKeyHandler,

		// Delivery
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
