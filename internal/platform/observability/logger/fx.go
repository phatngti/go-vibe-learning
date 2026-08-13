package logger

import (
	"context"

	"github/master_go_cqrs/internal/platform/config"

	"go.uber.org/fx"
)

var Module = fx.Module("logger",
	fx.Provide(New),
)

func New(lifecycle fx.Lifecycle, cfg *config.Logger) Factory {
	log := NewapiLogger(cfg)
	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return log.Sync()
		},
	})
	return NewFactory(log)
}
