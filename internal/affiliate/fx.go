package affiliate

import (
	"github/master_go_cqrs/internal/affiliate/application/command"
	"github/master_go_cqrs/internal/affiliate/application/query"
	"github/master_go_cqrs/internal/affiliate/delivery/grpc"
	"github/master_go_cqrs/internal/affiliate/delivery/http"
	"github/master_go_cqrs/internal/affiliate/repository/postgres"
	"github/master_go_cqrs/internal/httpapi/route"

	"go.uber.org/fx"
)

var Module = fx.Module("affiliate",
	fx.Provide(
		postgres.NewAffiliateRepository,
		command.NewRegisterReferralHandler,
		command.NewAccumulateVolumeHandler,
		query.NewGetRankHandler,
		query.NewGetDownlineHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	fx.Invoke(grpc.RequireServerHandler),
)
