package settlement

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
	settlementpostgres "github/master_go_cqrs/services/settlement/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/settlement/internal/application/command"
	"github/master_go_cqrs/services/settlement/internal/application/query"
	"github/master_go_cqrs/services/settlement/internal/config"
	settlementconsumer "github/master_go_cqrs/services/settlement/internal/transport/consumer"
	"github/master_go_cqrs/services/settlement/internal/transport/grpc"
	"github/master_go_cqrs/services/settlement/internal/transport/http"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("settlement",
	fx.Provide(
		// Ports → Adapters
		settlementpostgres.NewSettlementRepository,
		settlementpostgres.NewUnitOfWork,

		// Application — Commands
		command.NewCalculateAndCreditCommissionHandler,
		command.NewRequestWithdrawalHandler,
		command.NewAdjustCreditLimitHandler,

		// Application — Queries
		query.NewGetBalanceHandler,

		// Delivery
		settlementconsumer.NewCommissionCommandHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	kafka.AsSubscription(settlementconsumer.NewCommissionSubscription),
	fx.Invoke(grpc.RequireServerHandler),
)

var RuntimeModule = fx.Module("settlement-runtime",
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
