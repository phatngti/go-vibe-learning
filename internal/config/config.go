package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	OrchestratorEngineSaga     = "saga"
	OrchestratorEngineTemporal = "temporal"
)

type KafkaTopics struct {
	PaymentInitiatedEvents string `yaml:"paymentInitiatedEvents" env:"KAFKA_TOPIC_PAYMENT_INITIATED_EVENTS" env-default:"payments.initiated"`
	PaymentEvents          string `yaml:"paymentEvents" env:"KAFKA_TOPIC_PAYMENT_EVENTS" env-default:"payments.events"`
	AffiliateCommands  string `yaml:"affiliateCommands" env:"KAFKA_TOPIC_AFFILIATE_COMMANDS" env-default:"affiliate.commands"`
	AffiliateEvents    string `yaml:"affiliateEvents" env:"KAFKA_TOPIC_AFFILIATE_EVENTS" env-default:"affiliate.events"`
	SettlementCommands string `yaml:"settlementCommands" env:"KAFKA_TOPIC_SETTLEMENT_COMMANDS" env-default:"settlement.commands"`
	SettlementEvents   string `yaml:"settlementEvents" env:"KAFKA_TOPIC_SETTLEMENT_EVENTS" env-default:"settlement.events"`
	AgentCommands      string `yaml:"agentCommands" env:"KAFKA_TOPIC_AGENT_COMMANDS" env-default:"agent.commands"`
	AgentEvents        string `yaml:"agentEvents" env:"KAFKA_TOPIC_AGENT_EVENTS" env-default:"agent.events"`
	UserEvents         string `yaml:"userEvents" env:"KAFKA_TOPIC_USER_EVENTS" env-default:"user.events"`
}

type Config struct {
	Postgres struct {
		Host     string `yaml:"host" env:"POSTGRES_HOST" env-default:"127.0.0.1"`
		Port     string `yaml:"port" env:"POSTGRES_PORT" env-default:"5432"`
		User     string `yaml:"user" env:"POSTGRES_USER" env-default:"postgres"`
		Password string `yaml:"password" env:"POSTGRES_PASSWORD" env-default:"password"`
		DBName   string `yaml:"dbname" env:"POSTGRES_DBNAME" env-default:"postgres"`
	} `yaml:"postgres"`

	Logger struct {
		LogLevel    string `yaml:"logLevel" env-default:"info"`
		Mode        string `yaml:"mode" env-default:"development"`
		LogEncoding string `yaml:"encoding" env-default:"console"`
	} `yaml:"logger"`

	Server struct {
		Port int `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
	} `yaml:"server"`

	Gateway struct {
		PublicBaseURL string `yaml:"publicBaseUrl" env:"GATEWAY_PUBLIC_BASE_URL" env-default:"http://localhost:8888"`
	} `yaml:"gateway"`

	Keycloak struct {
		Endpoint     string `yaml:"endpoint" env:"KEYCLOAK_ENDPOINT" env-default:"http://localhost:8080"`
		ClientID     string `yaml:"clientId" env:"KEYCLOAK_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" env:"KEYCLOAK_CLIENT_SECRET"`
		Realm        string `yaml:"realm" env:"KEYCLOAK_REALM"`
		CustomHeader string `yaml:"customHeaderName" env:"KEYCLOAK_CUSTOM_HEADER_NAME"`
	} `yaml:"keycloak"`

	Otel struct {
		Endpoint string `yaml:"endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:"localhost:4318"`
		Service  string `yaml:"service" env:"OTEL_SERVICE_NAME" env-default:"app"`
	} `yaml:"otel"`

	Kafka struct {
		Brokers []string    `yaml:"brokers" env:"KAFKA_BROKERS" env-separator:"," env-default:"localhost:9092"`
		Topics  KafkaTopics `yaml:"topics"`
	} `yaml:"kafka"`

	Orchestrator struct {
		Engine   string `yaml:"engine" env:"ORCHESTRATOR_ENGINE" env-default:"saga"`
		Temporal struct {
			Address   string `yaml:"address" env:"TEMPORAL_ADDRESS" env-default:"localhost:7233"`
			Namespace string `yaml:"namespace" env:"TEMPORAL_NAMESPACE" env-default:"master-go-cqrs"`
			TaskQueue string `yaml:"taskQueue" env:"TEMPORAL_TASK_QUEUE" env-default:"payment-orchestration"`
		} `yaml:"temporal"`
	} `yaml:"orchestrator"`
}

func NewConfig() (*Config, error) {
	var config Config
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		var err error
		configPath, err = defaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	if err := cleanenv.ReadConfig(configPath, &config); err != nil {
		return nil, err
	}
	if tracesEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); tracesEndpoint != "" {
		config.Otel.Endpoint = tracesEndpoint
	}
	if err := config.ValidateOrchestrator(); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Config) ValidateOrchestrator() error {
	c.Orchestrator.Engine = strings.ToLower(strings.TrimSpace(c.Orchestrator.Engine))
	switch c.Orchestrator.Engine {
	case OrchestratorEngineSaga:
		return nil
	case OrchestratorEngineTemporal:
		if strings.TrimSpace(c.Orchestrator.Temporal.Address) == "" {
			return fmt.Errorf("orchestrator temporal address is required")
		}
		if strings.TrimSpace(c.Orchestrator.Temporal.Namespace) == "" {
			return fmt.Errorf("orchestrator temporal namespace is required")
		}
		if strings.TrimSpace(c.Orchestrator.Temporal.TaskQueue) == "" {
			return fmt.Errorf("orchestrator temporal task queue is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported orchestrator engine %q: expected %q or %q",
			c.Orchestrator.Engine, OrchestratorEngineSaga, OrchestratorEngineTemporal)
	}
}

func defaultConfigPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "app"
	}
	return filepath.Join(cwd, "config", serviceName+".yaml"), nil
}
