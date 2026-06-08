package settlement

import (
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/settlement/application/command"
	"github/master_go_cqrs/internal/settlement/application/query"
	"github/master_go_cqrs/internal/settlement/delivery/grpc"
	"github/master_go_cqrs/internal/settlement/delivery/http"
	"github/master_go_cqrs/internal/settlement/repository/postgres"

	"go.uber.org/fx"
)

var Module = fx.Module("settlement",
	fx.Provide(
		postgres.NewSettlementRepository,
		command.NewCalculateAndCreditCommissionHandler,
		command.NewRequestWithdrawalHandler,
		command.NewAdjustCreditLimitHandler,
		query.NewGetBalanceHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	fx.Invoke(grpc.RequireServerHandler),
)
