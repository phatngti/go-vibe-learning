package orchestrator

import (
	"time"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	"github/master_go_cqrs/internal/orchestrator/features/payment/codec"
	paymentsaga "github/master_go_cqrs/internal/orchestrator/features/payment/saga"
	paymentpostgres "github/master_go_cqrs/internal/orchestrator/features/payment/saga/repository/postgres"
	temporalengine "github/master_go_cqrs/internal/orchestrator/features/payment/temporal"
	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var SagaModule = fx.Module("orchestrator-saga",
	fx.Provide(
		codec.NewKafka,
		paymentpostgres.NewRepository,
		paymentsaga.NewProcess,
		paymentsaga.NewHandler,
		paymentsaga.NewReconciler,
	),
	kafka.AsSubscription(paymentsaga.NewPaymentInitiatedSubscription),
	kafka.AsSubscription(paymentsaga.NewPaymentSubscription),
	kafka.AsSubscription(paymentsaga.NewAffiliateSubscription),
	kafka.AsSubscription(paymentsaga.NewSettlementSubscription),
	kafka.AsSubscription(paymentsaga.NewAgentSubscription),
	fx.Invoke((*paymentsaga.Reconciler).Start),
)

func Options(cfg *config.Config) (fx.Option, error) {
	if err := cfg.ValidateOrchestrator(); err != nil {
		return nil, err
	}
	common := fx.Options(
		fx.Supply(cfg),
		logger.Module,
		telemetry.Module,
		kafka.ProducerModule,
		kafka.ConsumerModule,
	)
	var engine fx.Option
	switch cfg.Orchestrator.Engine {
	case config.OrchestratorEngineSaga:
		engine = fx.Options(platformpostgres.Module, inbox.Module, outbox.Module, SagaModule)
	case config.OrchestratorEngineTemporal:
		engine = temporalengine.Module
	}
	return fx.Options(common, engine, fx.StartTimeout(lifecycleTimeout), fx.StopTimeout(lifecycleTimeout)), nil
}
