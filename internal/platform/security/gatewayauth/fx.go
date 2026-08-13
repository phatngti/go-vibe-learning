package gatewayauth

import "go.uber.org/fx"

var Module = fx.Module("gatewayauth",
	fx.Provide(NewMiddleware),
)
