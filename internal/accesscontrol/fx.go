package accesscontrol

import (
	"github/master_go_cqrs/internal/accesscontrol/application/query"
	accesscontrolhttp "github/master_go_cqrs/internal/accesscontrol/delivery/http"
	"github/master_go_cqrs/internal/httpapi/route"

	"go.uber.org/fx"
)

var Module = fx.Module("authorize",
	fx.Provide(
		query.NewCheckAuthnHandler,
		query.NewCheckRBACHandler,
		route.AsRegistrar(accesscontrolhttp.NewHandler),
	),
)
