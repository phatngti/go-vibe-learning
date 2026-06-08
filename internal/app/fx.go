package app

import (
	"time"

	"github/master_go_cqrs/internal/accesscontrol"
	"github/master_go_cqrs/internal/app/application/command"
	"github/master_go_cqrs/internal/app/application/query"
	"github/master_go_cqrs/internal/app/delivery/http"
	"github/master_go_cqrs/internal/app/gateway/keycloak"
	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/httpapi"
	"github/master_go_cqrs/internal/httpapi/route"
	"github/master_go_cqrs/internal/observability/logger"
	"github/master_go_cqrs/internal/observability/telemetry"
	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
	"github/master_go_cqrs/internal/security/gatewayauth"

	"go.uber.org/fx"
)

const lifecycleTimeout = 30 * time.Second

var Module = fx.Module("app",
	fx.Provide(
		keycloak.NewAuthProvider,
		command.NewAuthentication,
		query.NewAuthenticationQuery,
		query.NewVerifyTokenHandler,
		query.NewGetInfoHandler,
		route.AsRegistrar(http.NewHandler),
	),
)

var RuntimeModule = fx.Module("app-runtime",
	config.Module,
	logger.Module,
	telemetry.Module,
	platformkeycloak.Module,
	gatewayauth.Module,
	Module,
	accesscontrol.Module,
	httpapi.Module,
)

func Options() fx.Option {
	return fx.Options(
		RuntimeModule,
		fx.StartTimeout(lifecycleTimeout),
		fx.StopTimeout(lifecycleTimeout),
	)
}
