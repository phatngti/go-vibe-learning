package user

import (
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/httpapi"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/kafka"
	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/observability/telemetry"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/user/internal/accesscontrol"
	userkeycloak "github/master_go_cqrs/services/user/internal/adapter/gateway/keycloak"
	userpostgres "github/master_go_cqrs/services/user/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/user/internal/application/command"
	"github/master_go_cqrs/services/user/internal/application/query"
	"github/master_go_cqrs/services/user/internal/config"
	userconsumer "github/master_go_cqrs/services/user/internal/transport/consumer"
	usergrpc "github/master_go_cqrs/services/user/internal/transport/grpc"
	"github/master_go_cqrs/services/user/internal/transport/http"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("user",
	fx.Provide(
		// Ports → Adapters
		userkeycloak.NewAuthProvider,
		userkeycloak.NewSyncGateway,
		userpostgres.NewUserRepository,
		userpostgres.NewUserSyncJobRepository,
		userpostgres.NewUnitOfWork,
		func(cfg *config.Config) command.UserSyncJobPolicy {
			return command.UserSyncJobPolicy{
				PageSize: cfg.SyncJobs.PageSize, LeaseDuration: cfg.SyncJobs.LeaseDuration,
				ProviderTimeout: cfg.SyncJobs.ProviderTimeout, MaxAttempts: cfg.SyncJobs.MaxAttempts,
				RetryBaseDelay: cfg.SyncJobs.RetryBaseDelay, RetryMaxDelay: cfg.SyncJobs.RetryMaxDelay,
			}
		},

		// Application — Authentication commands
		command.NewAuthentication,
		command.NewEmitUserRegisteredHandler,

		// Application — Authentication queries
		query.NewAuthenticationQuery,
		query.NewVerifyTokenHandler,
		query.NewGetInfoHandler,

		// Application — User management
		command.NewSyncUsersHandler,
		command.NewRequestUserSyncHandler,
		command.NewUserSyncJobRunner,
		query.NewGetUserHandler,
		query.NewGetUserByProviderIDHandler,
		query.NewListUsersHandler,

		// Delivery — HTTP
		route.AsRegistrar(http.NewHandler),

		// Delivery — gRPC
		usergrpc.NewServerHandler,

		// Delivery — Event-store consumer
		eventstore.NewConsumer,
		userconsumer.NewUserRegisteredHandler,
		userconsumer.NewUserRegisteredWorker,
		userconsumer.NewUserSyncJobWorker,
	),
	fx.Invoke(
		usergrpc.RequireServerHandler,
		(*userconsumer.UserRegisteredWorker).Start,
		(*userconsumer.UserSyncJobWorker).Start,
	),
)

var RuntimeModule = fx.Module("user-runtime",
	config.Module,
	logger.Module,
	telemetry.Module,
	platformkeycloak.Module,
	platformpostgres.Module,
	gatewayauth.Module,

	// Event store with outbox relay — UserRegistered and UserSynced events are
	// automatically forwarded from the event store to the outbox and then published to Kafka.
	eventstore.ModuleWithRelay,
	fx.Provide(func(cfg *config.Config) eventstore.RelayConfig {
		return eventstore.RelayConfig{
			messagebus.UserRegisteredType: {
				Topic:       cfg.Kafka.Topics.UserEvents,
				MessageType: messagebus.UserRegisteredType,
				StepName:    "user-registered",
			},
			messagebus.UserSyncedType: {
				Topic:       cfg.Kafka.Topics.UserEvents,
				MessageType: messagebus.UserSyncedType,
				StepName:    "user-synced",
			},
		}
	}),
	kafka.ProducerModule,
	outbox.Module,

	Module,
	accesscontrol.Module,
	httpapi.Module,
	fx.Provide(platformgrpc.NewServer, platformgrpc.ProvideRawServer),
)

func Options() fx.Option {
	return fx.Options(
		RuntimeModule,
		fx.StartTimeout(lifecycleTimeout),
		fx.StopTimeout(lifecycleTimeout),
	)
}
