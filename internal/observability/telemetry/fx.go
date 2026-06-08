package telemetry

import (
	"context"

	"github/master_go_cqrs/internal/config"

	"go.uber.org/fx"
)

var Module = fx.Module("telemetry",
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lifecycle fx.Lifecycle, cfg *config.Config) {
	var shutdown func(context.Context) error
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
			shutdown, err = SetUpOtelSdk(cfg, ctx)
			return err
		},
		OnStop: func(ctx context.Context) error {
			if shutdown == nil {
				return nil
			}
			return shutdown(ctx)
		},
	})
}
