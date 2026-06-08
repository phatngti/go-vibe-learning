package tokenauth

import "go.uber.org/fx"

var Module = fx.Module("tokenauth",
	fx.Provide(NewMiddleware),
)
