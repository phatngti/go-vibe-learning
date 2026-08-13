package config

import (
	platformconfig "github/master_go_cqrs/internal/platform/config"
	"go.uber.org/fx"
)

type Config struct {
	Postgres platformconfig.Postgres `yaml:"postgres"`
	Logger   platformconfig.Logger   `yaml:"logger"`
	Server   platformconfig.Server   `yaml:"server"`
	Otel     platformconfig.Otel     `yaml:"otel"`
	Kafka    platformconfig.Kafka    `yaml:"kafka"`
}

func New() (*Config, error) { var cfg Config; return &cfg, platformconfig.Load("settlement", &cfg) }

type PlatformComponents struct {
	fx.Out
	Postgres *platformconfig.Postgres
	Logger   *platformconfig.Logger
	Server   *platformconfig.Server
	Otel     *platformconfig.Otel
	Kafka    *platformconfig.Kafka
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{Postgres: &cfg.Postgres, Logger: &cfg.Logger, Server: &cfg.Server, Otel: &cfg.Otel, Kafka: &cfg.Kafka}
}

var Module = fx.Module("settlement-config", fx.Provide(New, Components))
