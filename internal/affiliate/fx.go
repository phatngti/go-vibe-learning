package affiliate

import (
	"time"

	"github/master_go_cqrs/internal/affiliate/application/command"
	"github/master_go_cqrs/internal/affiliate/application/query"
	affiliateconsumer "github/master_go_cqrs/internal/affiliate/consumer"
	"github/master_go_cqrs/internal/affiliate/delivery/grpc"
	"github/master_go_cqrs/internal/affiliate/delivery/http"
	"github/master_go_cqrs/internal/affiliate/repository/postgres"
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

var Module = fx.Module("affiliate",
	fx.Provide(
		postgres.NewAffiliateRepository,
		command.NewRegisterReferralHandler,
		command.NewAccumulateVolumeHandler,
		query.NewGetRankHandler,
		query.NewGetDownlineHandler,
		affiliateconsumer.NewVolumeCommandHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	kafka.AsSubscription(affiliateconsumer.NewVolumeSubscription),
	fx.Invoke(grpc.RequireServerHandler),
)

var RuntimeModule = fx.Module("affiliate-runtime",
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
