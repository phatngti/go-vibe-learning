package config

import (
	platformconfig "github/master_go_cqrs/internal/platform/config"
	"go.uber.org/fx"
)

type Config struct {
	Postgres platformconfig.Postgres `yaml:"postgres"`
	Logger   platformconfig.Logger   `yaml:"logger"`
	Server   platformconfig.Server   `yaml:"server"`
	Keycloak platformconfig.Keycloak `yaml:"keycloak"`
	Otel     platformconfig.Otel     `yaml:"otel"`
	Kafka    platformconfig.Kafka    `yaml:"kafka"`
}

func New() (*Config, error) { var cfg Config; return &cfg, platformconfig.Load("user", &cfg) }

type PlatformComponents struct {
	fx.Out
	Postgres *platformconfig.Postgres
	Logger   *platformconfig.Logger
	Server   *platformconfig.Server
	Keycloak *platformconfig.Keycloak
	Otel     *platformconfig.Otel
	Kafka    *platformconfig.Kafka
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{Postgres: &cfg.Postgres, Logger: &cfg.Logger, Server: &cfg.Server, Keycloak: &cfg.Keycloak, Otel: &cfg.Otel, Kafka: &cfg.Kafka}
}

var Module = fx.Module("user-config", fx.Provide(New, Components))
