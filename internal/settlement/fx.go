package settlement

import (
	"time"

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
	"github/master_go_cqrs/internal/settlement/application/command"
	"github/master_go_cqrs/internal/settlement/application/query"
	settlementconsumer "github/master_go_cqrs/internal/settlement/consumer"
	"github/master_go_cqrs/internal/settlement/delivery/grpc"
	"github/master_go_cqrs/internal/settlement/delivery/http"
	"github/master_go_cqrs/internal/settlement/repository/postgres"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("settlement",
	fx.Provide(
		postgres.NewSettlementRepository,
		command.NewCalculateAndCreditCommissionHandler,
		command.NewRequestWithdrawalHandler,
		command.NewAdjustCreditLimitHandler,
		query.NewGetBalanceHandler,
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
