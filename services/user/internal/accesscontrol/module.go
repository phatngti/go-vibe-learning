package accesscontrol

import (
	"github/master_go_cqrs/internal/platform/httpapi/route"
	"github/master_go_cqrs/services/user/internal/accesscontrol/application/query"
	accesscontrolhttp "github/master_go_cqrs/services/user/internal/accesscontrol/transport/http"

	"go.uber.org/fx"
)

var Module = fx.Module("authorize",
	fx.Provide(
		query.NewCheckAuthnHandler,
		query.NewCheckRBACHandler,
		route.AsRegistrar(accesscontrolhttp.NewHandler),
	),
)
