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
	Outbox       platformconfig.Outbox   `yaml:"outbox"`
	Orchestrator struct {
		Engine     string `yaml:"engine" env:"ORCHESTRATOR_ENGINE" env-default:"saga"`
		Owner      string `yaml:"owner" env:"ORCHESTRATOR_OWNER"`
		Generation int64  `yaml:"generation" env:"ORCHESTRATOR_GENERATION" env-default:"1"`
		Temporal   struct {
			Address           string `yaml:"address" env:"TEMPORAL_ADDRESS" env-default:"localhost:7233"`
			Namespace         string `yaml:"namespace" env:"TEMPORAL_NAMESPACE" env-default:"master-go-cqrs"`
			TaskQueue         string `yaml:"taskQueue" env:"TEMPORAL_TASK_QUEUE" env-default:"payment-orchestration"`
			TLSEnabled        bool   `yaml:"tlsEnabled" env:"TEMPORAL_TLS_ENABLED" env-default:"false"`
			TLSCAFile         string `yaml:"tlsCAFile" env:"TEMPORAL_TLS_CA_FILE"`
			TLSCertFile       string `yaml:"tlsCertFile" env:"TEMPORAL_TLS_CERT_FILE"`
			TLSKeyFile        string `yaml:"tlsKeyFile" env:"TEMPORAL_TLS_KEY_FILE"`
			TLSServerName     string `yaml:"tlsServerName" env:"TEMPORAL_TLS_SERVER_NAME"`
			VersioningEnabled bool   `yaml:"versioningEnabled" env:"TEMPORAL_VERSIONING_ENABLED" env-default:"false"`
			DeploymentName    string `yaml:"deploymentName" env:"TEMPORAL_DEPLOYMENT_NAME"`
			BuildID           string `yaml:"buildId" env:"TEMPORAL_BUILD_ID"`
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
	if c.Orchestrator.Generation == 0 {
		c.Orchestrator.Generation = 1
	}
	if c.Orchestrator.Generation < 1 {
		return fmt.Errorf("orchestration generation must be positive")
	}
	expectedOwner := fmt.Sprintf("%s:v%d", c.Orchestrator.Engine, c.Orchestrator.Generation)
	c.Orchestrator.Owner = strings.ToLower(strings.TrimSpace(c.Orchestrator.Owner))
	if c.Orchestrator.Owner == "" {
		c.Orchestrator.Owner = expectedOwner
	}
	if c.Orchestrator.Owner != expectedOwner {
		return fmt.Errorf("orchestration owner %q must match engine and generation %q", c.Orchestrator.Owner, expectedOwner)
	}
	switch c.Orchestrator.Engine {
	case OrchestratorEngineSaga:
		return nil
	case OrchestratorEngineTemporal:
		settings := c.Orchestrator.Temporal
		if strings.TrimSpace(settings.Address) == "" || strings.TrimSpace(settings.Namespace) == "" || strings.TrimSpace(settings.TaskQueue) == "" {
			return fmt.Errorf("temporal address, namespace, and task queue are required")
		}
		if (strings.TrimSpace(settings.TLSCertFile) == "") != (strings.TrimSpace(settings.TLSKeyFile) == "") {
			return fmt.Errorf("temporal TLS certificate and key must be configured together")
		}
		if settings.VersioningEnabled && (strings.TrimSpace(settings.DeploymentName) == "" || strings.TrimSpace(settings.BuildID) == "") {
			return fmt.Errorf("temporal deployment name and build ID are required when worker versioning is enabled")
		}
		return nil
	default:
		return fmt.Errorf("unsupported orchestrator engine %q", c.Orchestrator.Engine)
	}
}

func (c *Config) OrchestrationIdentity() (string, int64) {
	engine := strings.ToLower(strings.TrimSpace(c.Orchestrator.Engine))
	if engine == "" {
		engine = OrchestratorEngineSaga
	}
	generation := c.Orchestrator.Generation
	if generation < 1 {
		generation = 1
	}
	owner := strings.ToLower(strings.TrimSpace(c.Orchestrator.Owner))
	if owner == "" {
		owner = fmt.Sprintf("%s:v%d", engine, generation)
	}
	return owner, generation
}

// ConsumerGroupID isolates orchestration engines and generations. The original
// Saga v1 names are retained so an upgrade does not replay retained topics just
// because the group name changed.
func (c *Config) ConsumerGroupID(stream string) string {
	engine := strings.ToLower(strings.TrimSpace(c.Orchestrator.Engine))
	if engine == "" {
		engine = OrchestratorEngineSaga
	}
	_, generation := c.OrchestrationIdentity()
	if engine == OrchestratorEngineSaga && generation == 1 {
		return "orchestrator-" + stream
	}
	return fmt.Sprintf("orchestrator-%s-v%d-%s", engine, generation, stream)
}

type PlatformComponents struct {
	fx.Out
	Postgres *platformconfig.Postgres
	Logger   *platformconfig.Logger
	Otel     *platformconfig.Otel
	Kafka    *platformconfig.Kafka
	Outbox   *platformconfig.Outbox
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{Postgres: &cfg.Postgres, Logger: &cfg.Logger, Otel: &cfg.Otel, Kafka: &cfg.Kafka, Outbox: &cfg.Outbox}
}

var Module = fx.Module("orchestrator-config", fx.Provide(New, Components))
