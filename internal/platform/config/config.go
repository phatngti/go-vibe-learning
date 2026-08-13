package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Postgres struct {
	Host     string `yaml:"host" env:"POSTGRES_HOST" env-default:"127.0.0.1"`
	Port     string `yaml:"port" env:"POSTGRES_PORT" env-default:"5432"`
	User     string `yaml:"user" env:"POSTGRES_USER" env-default:"postgres"`
	Password string `yaml:"password" env:"POSTGRES_PASSWORD" env-default:"password"`
	DBName   string `yaml:"dbname" env:"POSTGRES_DBNAME" env-default:"postgres"`
}

type Logger struct {
	LogLevel    string `yaml:"logLevel" env-default:"info"`
	Mode        string `yaml:"mode" env-default:"development"`
	LogEncoding string `yaml:"encoding" env-default:"console"`
}

type Server struct {
	Port int `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
}

type Gateway struct {
	PublicBaseURL string `yaml:"publicBaseUrl" env:"GATEWAY_PUBLIC_BASE_URL" env-default:"http://localhost:8888"`
}

type Keycloak struct {
	Endpoint     string `yaml:"endpoint" env:"KEYCLOAK_ENDPOINT" env-default:"http://localhost:8080"`
	ClientID     string `yaml:"clientId" env:"KEYCLOAK_CLIENT_ID"`
	ClientSecret string `yaml:"clientSecret" env:"KEYCLOAK_CLIENT_SECRET"`
	Realm        string `yaml:"realm" env:"KEYCLOAK_REALM"`
	CustomHeader string `yaml:"customHeaderName" env:"KEYCLOAK_CUSTOM_HEADER_NAME"`
}

type Otel struct {
	Endpoint string `yaml:"endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:"localhost:4318"`
	Service  string `yaml:"service" env:"OTEL_SERVICE_NAME" env-default:"app"`
}

type KafkaTopics struct {
	PaymentInitiatedEvents string `yaml:"paymentInitiatedEvents" env:"KAFKA_TOPIC_PAYMENT_INITIATED_EVENTS" env-default:"payments.initiated"`
	PaymentEvents          string `yaml:"paymentEvents" env:"KAFKA_TOPIC_PAYMENT_EVENTS" env-default:"payments.events"`
	AffiliateCommands      string `yaml:"affiliateCommands" env:"KAFKA_TOPIC_AFFILIATE_COMMANDS" env-default:"affiliate.commands"`
	AffiliateEvents        string `yaml:"affiliateEvents" env:"KAFKA_TOPIC_AFFILIATE_EVENTS" env-default:"affiliate.events"`
	SettlementCommands     string `yaml:"settlementCommands" env:"KAFKA_TOPIC_SETTLEMENT_COMMANDS" env-default:"settlement.commands"`
	SettlementEvents       string `yaml:"settlementEvents" env:"KAFKA_TOPIC_SETTLEMENT_EVENTS" env-default:"settlement.events"`
	AgentCommands          string `yaml:"agentCommands" env:"KAFKA_TOPIC_AGENT_COMMANDS" env-default:"agent.commands"`
	AgentEvents            string `yaml:"agentEvents" env:"KAFKA_TOPIC_AGENT_EVENTS" env-default:"agent.events"`
	UserEvents             string `yaml:"userEvents" env:"KAFKA_TOPIC_USER_EVENTS" env-default:"user.events"`
}

type Kafka struct {
	Brokers []string    `yaml:"brokers" env:"KAFKA_BROKERS" env-separator:"," env-default:"localhost:9092"`
	Topics  KafkaTopics `yaml:"topics"`
}

func Load(service string, target any) error {
	path := strings.TrimSpace(os.Getenv("CONFIG_PATH"))
	if path == "" {
		path = filepath.Join("services", service, "config", service+".yaml")
	}
	if err := cleanenv.ReadConfig(path, target); err != nil {
		return err
	}
	if endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")); endpoint != "" {
		_ = os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
		return cleanenv.ReadEnv(target)
	}
	return nil
}
