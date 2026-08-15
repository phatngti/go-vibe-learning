package config

import (
	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"go.uber.org/fx"
)

type Config struct {
	Postgres       platformconfig.Postgres        `yaml:"postgres"`
	Logger         platformconfig.Logger          `yaml:"logger"`
	Server         platformconfig.Server          `yaml:"server"`
	Otel           platformconfig.Otel            `yaml:"otel"`
	Kafka          platformconfig.Kafka           `yaml:"kafka"`
	Outbox         platformconfig.Outbox          `yaml:"outbox"`
	Authentication gatewayauth.VerificationConfig `yaml:"authentication"`
}

func New() (*Config, error) { var cfg Config; return &cfg, platformconfig.Load("affiliate", &cfg) }

type PlatformComponents struct {
	fx.Out
	Postgres       *platformconfig.Postgres
	Logger         *platformconfig.Logger
	Server         *platformconfig.Server
	Otel           *platformconfig.Otel
	Kafka          *platformconfig.Kafka
	Outbox         *platformconfig.Outbox
	Authentication *gatewayauth.VerificationConfig
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{Postgres: &cfg.Postgres, Logger: &cfg.Logger, Server: &cfg.Server, Otel: &cfg.Otel, Kafka: &cfg.Kafka, Outbox: &cfg.Outbox, Authentication: &cfg.Authentication}
}

var Module = fx.Module("affiliate-config", fx.Provide(New, Components))
