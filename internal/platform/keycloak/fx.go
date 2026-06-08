package keycloak

import "go.uber.org/fx"

var Module = fx.Module("keycloak",
	fx.Provide(NewClient),
)
