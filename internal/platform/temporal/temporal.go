package temporal

import (
	"context"
	"os"

	"github/master_go_cqrs/internal/config"
	"github/master_go_cqrs/internal/observability/logger"

	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("temporal",
	fx.Provide(NewClient),
)

func NewClient(lifecycle fx.Lifecycle, cfg *config.Config, l logger.Factory) (client.Client, error) {
	l.Bg().Info("Connecting to Temporal Server", zap.String("endpoint", cfg.Otel.Endpoint)) // Using Endpoint or default

	// Use TEMPORAL_HOSTPORT env var or default to localhost:7233
	hostPort := os.Getenv("TEMPORAL_HOSTPORT")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		l.Bg().Error("Failed to connect to Temporal server", zap.Error(err))
		return nil, err
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			l.Bg().Info("Closing Temporal client connection")
			c.Close()
			return nil
		},
	})

	return c, nil
}
