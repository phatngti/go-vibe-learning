package temporal

import (
	"context"
	"fmt"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func NewClient(lifecycle fx.Lifecycle, cfg *config.Config, l logger.Factory) (client.Client, error) {
	settings := cfg.Orchestrator.Temporal
	l.Bg().Info("Connecting to Temporal", zap.String("address", settings.Address), zap.String("namespace", settings.Namespace))
	c, err := client.Dial(client.Options{HostPort: settings.Address, Namespace: settings.Namespace})
	if err != nil {
		return nil, fmt.Errorf("connect to Temporal: %w", err)
	}
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		c.Close()
		return nil
	}})
	return c, nil
}
