// File: internal/platform/inbox/fx.go
// Description: Fx dependency injection module configuration for the message inbox package.

package inbox

import "go.uber.org/fx"

var Module = fx.Module("inbox", fx.Provide(NewRepository))
