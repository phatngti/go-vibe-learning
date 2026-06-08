package payment

import (
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/payment/application/command"
	"github/master_go_cqrs/internal/payment/application/query"
	"github/master_go_cqrs/internal/payment/delivery/grpc"
	"github/master_go_cqrs/internal/payment/delivery/http"
	"github/master_go_cqrs/internal/payment/gateway/simulator"
	"github/master_go_cqrs/internal/payment/repository/postgres"

	"go.uber.org/fx"
)

var Module = fx.Module("payment",
	fx.Provide(
		postgres.NewTransactionRepository,
		simulator.NewSimulatorGateway,
		command.NewInitiatePaymentHandler,
		command.NewProcessPaymentHandler,
		query.NewGetTransactionHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	fx.Invoke(grpc.RequireServerHandler),
)
