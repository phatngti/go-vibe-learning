package user

import (
	"time"

	"github/master_go_cqrs/internal/accesscontrol"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/kafka"
	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/security/gatewayauth"
	"github/master_go_cqrs/internal/user/application/command"
	"github/master_go_cqrs/internal/user/application/query"
	userconsumer "github/master_go_cqrs/internal/user/consumer"
	usergrpc "github/master_go_cqrs/internal/user/delivery/grpc"
	"github/master_go_cqrs/internal/user/delivery/http"
	userkeycloak "github/master_go_cqrs/internal/user/gateway/keycloak"
	"github/master_go_cqrs/internal/user/repository/postgres"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("user",
	fx.Provide(
		// Authentication (existing)
		userkeycloak.NewAuthProvider,
		command.NewAuthentication,
		query.NewAuthenticationQuery,
		query.NewVerifyTokenHandler,
		query.NewGetInfoHandler,
		route.AsRegistrar(http.NewHandler),

		// User management
		postgres.NewUserRepository,
		userkeycloak.NewSyncGateway,
		command.NewSyncUsersHandler,
		query.NewGetUserHandler,
		query.NewGetUserByProviderIDHandler,
		query.NewListUsersHandler,
		usergrpc.NewServerHandler,

		// Event-store consumer for UserRegistered events
		eventstore.NewConsumer,
		userconsumer.NewUserRegisteredHandler,
		userconsumer.NewUserRegisteredWorker,
	),
	fx.Invoke(
		usergrpc.RequireServerHandler,
		(*userconsumer.UserRegisteredWorker).Start,
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
