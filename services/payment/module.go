package payment

import (
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/eventstore"
	platformgrpc "github/master_go_cqrs/internal/platform/grpc"
	"github/master_go_cqrs/internal/platform/httpapi"
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/internal/platform/observability/telemetry"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"github/master_go_cqrs/services/payment/internal/adapter/gateway/simulator"
	paymentpostgres "github/master_go_cqrs/services/payment/internal/adapter/persistence/postgres"
	"github/master_go_cqrs/services/payment/internal/application/command"
	"github/master_go_cqrs/services/payment/internal/application/query"
	"github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/transport/grpc"
	"github/master_go_cqrs/services/payment/internal/transport/http"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("payment",
	fx.Provide(
		// Ports → Adapters
		paymentpostgres.NewTransactionRepository,
		paymentpostgres.NewPaymentV2Repository,
		paymentpostgres.NewUnitOfWork,
		simulator.NewSimulatorGateway,
		newPaymentProvider,

		// Application — Commands
		command.NewInitiatePaymentHandler,
		command.NewProcessPaymentHandler,
		command.NewPaymentReconciler,
		command.NewCreatePaymentV2Handler,
		command.NewHandleProviderWebhookHandler,

		// Application — Queries
		query.NewGetTransactionHandler,
		query.NewGetTransactionHistoryHandler,
		query.NewGetPaymentV2Handler,

		// Delivery
		route.AsRegistrar(http.NewHandler),
		route.AsV2Registrar(http.NewV2Handler),
		grpc.NewServerHandler,
		grpc.NewV2ServerHandler,
	),
	fx.Invoke((*command.PaymentReconciler).Start),
	fx.Invoke(grpc.RequireServerHandler),
	fx.Invoke(grpc.RequireV2ServerHandler),
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
			messagebus.PaymentFailedType: {
				Topic:       cfg.Kafka.Topics.PaymentEvents,
				MessageType: messagebus.PaymentFailedType,
				StepName:    "payment-failed",
			},
			messagebus.PaymentProcessingUncertainType: {
				Topic:       cfg.Kafka.Topics.PaymentEvents,
				MessageType: messagebus.PaymentProcessingUncertainType,
				StepName:    "payment-processing-uncertain",
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
