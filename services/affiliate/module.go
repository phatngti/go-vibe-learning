package affiliate

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
	affiliatepostgres "github/master_go_cqrs/services/affiliate/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/affiliate/internal/application/command"
	"github/master_go_cqrs/services/affiliate/internal/application/query"
	"github/master_go_cqrs/services/affiliate/internal/config"
	affiliateconsumer "github/master_go_cqrs/services/affiliate/internal/transport/consumer"
	"github/master_go_cqrs/services/affiliate/internal/transport/grpc"
	"github/master_go_cqrs/services/affiliate/internal/transport/http"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("affiliate",
	fx.Provide(
		// Ports → Adapters
		affiliatepostgres.NewAffiliateRepository,
		affiliatepostgres.NewUnitOfWork,

		// Application — Commands
		command.NewRegisterReferralHandler,
		command.NewAccumulateVolumeHandler,

		// Application — Queries
		query.NewGetRankHandler,
		query.NewGetDownlineHandler,

		// Delivery
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
