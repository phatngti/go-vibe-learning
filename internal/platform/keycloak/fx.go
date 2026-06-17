// File: internal/platform/keycloak/fx.go
// Description: Fx dependency injection module configuration for the keycloak package.

package keycloak

import "go.uber.org/fx"

var Module = fx.Module("keycloak",
	fx.Provide(NewClient),
)
