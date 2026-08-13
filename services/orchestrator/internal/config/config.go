package config

import (
	"fmt"
	"strings"

	platformconfig "github/master_go_cqrs/internal/platform/config"
	"go.uber.org/fx"
)

const (
	OrchestratorEngineSaga     = "saga"
	OrchestratorEngineTemporal = "temporal"
)

type KafkaTopics = platformconfig.KafkaTopics

type Config struct {
	Postgres     platformconfig.Postgres `yaml:"postgres"`
	Logger       platformconfig.Logger   `yaml:"logger"`
	Otel         platformconfig.Otel     `yaml:"otel"`
	Kafka        platformconfig.Kafka    `yaml:"kafka"`
	Orchestrator struct {
		Engine   string `yaml:"engine" env:"ORCHESTRATOR_ENGINE" env-default:"saga"`
		Temporal struct {
			Address   string `yaml:"address" env:"TEMPORAL_ADDRESS" env-default:"localhost:7233"`
			Namespace string `yaml:"namespace" env:"TEMPORAL_NAMESPACE" env-default:"master-go-cqrs"`
			TaskQueue string `yaml:"taskQueue" env:"TEMPORAL_TASK_QUEUE" env-default:"payment-orchestration"`
		} `yaml:"temporal"`
	} `yaml:"orchestrator"`
}

func New() (*Config, error) {
	var cfg Config
	if err := platformconfig.Load("orchestrator", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.ValidateOrchestrator(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) ValidateOrchestrator() error {
	c.Orchestrator.Engine = strings.ToLower(strings.TrimSpace(c.Orchestrator.Engine))
	switch c.Orchestrator.Engine {
	case OrchestratorEngineSaga:
		return nil
	case OrchestratorEngineTemporal:
		if strings.TrimSpace(c.Orchestrator.Temporal.Address) == "" || strings.TrimSpace(c.Orchestrator.Temporal.Namespace) == "" || strings.TrimSpace(c.Orchestrator.Temporal.TaskQueue) == "" {
			return fmt.Errorf("temporal address, namespace, and task queue are required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported orchestrator engine %q", c.Orchestrator.Engine)
	}
}

type PlatformComponents struct {
	fx.Out
	Postgres *platformconfig.Postgres
	Logger   *platformconfig.Logger
	Otel     *platformconfig.Otel
	Kafka    *platformconfig.Kafka
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{Postgres: &cfg.Postgres, Logger: &cfg.Logger, Otel: &cfg.Otel, Kafka: &cfg.Kafka}
}

var Module = fx.Module("orchestrator-config", fx.Provide(New, Components))
