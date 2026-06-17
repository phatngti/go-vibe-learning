package payment

import (
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/payment/application/command"
	"github/master_go_cqrs/internal/payment/application/query"
	"github/master_go_cqrs/internal/payment/delivery/grpc"
	"github/master_go_cqrs/internal/payment/delivery/http"
	"github/master_go_cqrs/internal/payment/gateway/simulator"
	"github/master_go_cqrs/internal/payment/repository/postgres"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/messagebus"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("payment",
	fx.Provide(
		postgres.NewTransactionRepository,
		simulator.NewSimulatorGateway,
		command.NewInitiatePaymentHandler,
		command.NewProcessPaymentHandler,
		query.NewGetTransactionHandler,
		query.NewGetTransactionHistoryHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	fx.Invoke(grpc.RequireServerHandler),
)

var RuntimeModule = fx.Module("payment-runtime",
	config.Module,
	logger.Module,
	telemetry.Module,
	platformpostgres.Module,
	outbox.Module,
	eventstore.ModuleWithRelay,
	fx.Provide(func(cfg *config.Config) eventstore.RelayConfig {
		return eventstore.RelayConfig{
			messagebus.PaymentInitiatedType: {
				Topic:       cfg.Kafka.Topics.PaymentInitiatedEvents,
				MessageType: messagebus.PaymentInitiatedType,
				StepName:    "payment-initiated",
			},
			messagebus.PaymentCompletedType: {
				Topic:       cfg.Kafka.Topics.PaymentEvents,
				MessageType: messagebus.PaymentCompletedType,
				StepName:    "payment-completed",
			},
		}
	}),
	kafka.ProducerModule,
	gatewayauth.Module,
	Module,
	httpapi.Module,
	fx.Provide(platformgrpc.NewServer, platformgrpc.ProvideRawServer),
)

func Options() fx.Option {
	return fx.Options(RuntimeModule, fx.StartTimeout(lifecycleTimeout), fx.StopTimeout(lifecycleTimeout))
}
