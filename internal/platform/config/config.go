package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Postgres struct {
	Host             string        `yaml:"host" env:"POSTGRES_HOST" env-default:"127.0.0.1"`
	Port             string        `yaml:"port" env:"POSTGRES_PORT" env-default:"5432"`
	User             string        `yaml:"user" env:"POSTGRES_USER" env-default:"postgres"`
	Password         string        `yaml:"password" env:"POSTGRES_PASSWORD" env-default:"password"`
	DBName           string        `yaml:"dbname" env:"POSTGRES_DBNAME" env-default:"postgres"`
	SSLMode          string        `yaml:"sslMode" env:"POSTGRES_SSL_MODE" env-default:"disable"`
	SSLRootCert      string        `yaml:"sslRootCert" env:"POSTGRES_SSL_ROOT_CERT"`
	SSLCert          string        `yaml:"sslCert" env:"POSTGRES_SSL_CERT"`
	SSLKey           string        `yaml:"sslKey" env:"POSTGRES_SSL_KEY"`
	ApplicationName  string        `yaml:"applicationName" env:"POSTGRES_APPLICATION_NAME" env-default:"master-go-cqrs"`
	ConnectTimeout   time.Duration `yaml:"connectTimeout" env:"POSTGRES_CONNECT_TIMEOUT" env-default:"10s"`
	MaxOpenConns     int           `yaml:"maxOpenConns" env:"POSTGRES_MAX_OPEN_CONNS" env-default:"50"`
	MaxIdleConns     int           `yaml:"maxIdleConns" env:"POSTGRES_MAX_IDLE_CONNS" env-default:"10"`
	ConnMaxLifetime  time.Duration `yaml:"connMaxLifetime" env:"POSTGRES_CONN_MAX_LIFETIME" env-default:"30m"`
	ConnMaxIdleTime  time.Duration `yaml:"connMaxIdleTime" env:"POSTGRES_CONN_MAX_IDLE_TIME" env-default:"5m"`
	StatementTimeout time.Duration `yaml:"statementTimeout" env:"POSTGRES_STATEMENT_TIMEOUT" env-default:"30s"`
}

type Logger struct {
	LogLevel    string `yaml:"logLevel" env:"LOG_LEVEL" env-default:"info"`
	Mode        string `yaml:"mode" env:"APP_ENV" env-default:"development"`
	LogEncoding string `yaml:"encoding" env:"LOG_ENCODING" env-default:"console"`
}

type Server struct {
	Port              int           `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
	ReadHeaderTimeout time.Duration `yaml:"readHeaderTimeout" env:"SERVER_READ_HEADER_TIMEOUT" env-default:"5s"`
	ReadTimeout       time.Duration `yaml:"readTimeout" env:"SERVER_READ_TIMEOUT" env-default:"15s"`
	WriteTimeout      time.Duration `yaml:"writeTimeout" env:"SERVER_WRITE_TIMEOUT" env-default:"30s"`
	IdleTimeout       time.Duration `yaml:"idleTimeout" env:"SERVER_IDLE_TIMEOUT" env-default:"60s"`
	AllowedOrigins    []string      `yaml:"allowedOrigins" env:"SERVER_ALLOWED_ORIGINS" env-separator:"," env-default:"*"`
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
	Endpoint              string        `yaml:"endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-default:"localhost:4318"`
	MetricsEndpoint       string        `yaml:"metricsEndpoint" env:"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"`
	MetricsExportInterval time.Duration `yaml:"metricsExportInterval" env:"OTEL_METRIC_EXPORT_INTERVAL" env-default:"15s"`
	Service               string        `yaml:"service" env:"OTEL_SERVICE_NAME" env-default:"app"`
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
	Brokers                   []string            `yaml:"brokers" env:"KAFKA_BROKERS" env-separator:"," env-default:"localhost:9092"`
	Topics                    KafkaTopics         `yaml:"topics"`
	ConsumerMaxAttempts       int                 `yaml:"consumerMaxAttempts" env:"KAFKA_CONSUMER_MAX_ATTEMPTS" env-default:"8"`
	RetryMaxDelay             time.Duration       `yaml:"retryMaxDelay" env:"KAFKA_RETRY_MAX_DELAY" env-default:"30s"`
	FetchMaxBytes             int                 `yaml:"fetchMaxBytes" env:"KAFKA_FETCH_MAX_BYTES" env-default:"10485760"`
	SessionTimeout            time.Duration       `yaml:"sessionTimeout" env:"KAFKA_SESSION_TIMEOUT" env-default:"30s"`
	HeartbeatInterval         time.Duration       `yaml:"heartbeatInterval" env:"KAFKA_HEARTBEAT_INTERVAL" env-default:"3s"`
	DeadLetterMaxPayloadBytes int                 `yaml:"deadLetterMaxPayloadBytes" env:"KAFKA_DLQ_MAX_PAYLOAD_BYTES" env-default:"262144"`
	Producer                  KafkaProducer       `yaml:"producer"`
	Serialization             KafkaSerialization  `yaml:"serialization"`
	SchemaRegistry            KafkaSchemaRegistry `yaml:"schemaRegistry"`
	TLS                       KafkaTLS            `yaml:"tls"`
	SASL                      KafkaSASL           `yaml:"sasl"`
}

const (
	KafkaSerializationJSON     = "json"
	KafkaSerializationProtobuf = "protobuf"
)

// KafkaSerialization selects the wire format used by the platform producer
// and consumer. JSON remains the default so existing topics and consumers are
// not changed until their dedicated v2 topics are enabled explicitly.
type KafkaSerialization struct {
	Format string `yaml:"format" env:"KAFKA_SERIALIZATION_FORMAT" env-default:"json"`
}

// KafkaSchemaRegistry contains only connection and bounded-cache settings.
// Topic subjects are derived using the standard <topic>-value convention.
type KafkaSchemaRegistry struct {
	URL            string        `yaml:"url" env:"KAFKA_SCHEMA_REGISTRY_URL"`
	Username       string        `yaml:"username" env:"KAFKA_SCHEMA_REGISTRY_USERNAME"`
	Password       string        `yaml:"password" env:"KAFKA_SCHEMA_REGISTRY_PASSWORD"`
	Compatibility  string        `yaml:"compatibility" env:"KAFKA_SCHEMA_REGISTRY_COMPATIBILITY" env-default:"BACKWARD_TRANSITIVE"`
	RequestTimeout time.Duration `yaml:"requestTimeout" env:"KAFKA_SCHEMA_REGISTRY_REQUEST_TIMEOUT" env-default:"5s"`
	CacheTTL       time.Duration `yaml:"cacheTtl" env:"KAFKA_SCHEMA_REGISTRY_CACHE_TTL" env-default:"15m"`
	CacheCapacity  int           `yaml:"cacheCapacity" env:"KAFKA_SCHEMA_REGISTRY_CACHE_CAPACITY" env-default:"1024"`
}

// ValidateSerialization rejects partial Protobuf/Schema Registry
// configuration at startup. A registry failure at runtime is never converted
// into a JSON fallback.
func (k Kafka) ValidateSerialization() error {
	format := strings.ToLower(strings.TrimSpace(k.Serialization.Format))
	if format == "" {
		format = KafkaSerializationJSON
	}
	if format != KafkaSerializationJSON && format != KafkaSerializationProtobuf {
		return fmt.Errorf("kafka: unsupported serialization format %q", k.Serialization.Format)
	}
	if format == KafkaSerializationProtobuf && strings.TrimSpace(k.SchemaRegistry.URL) == "" {
		return fmt.Errorf("kafka: schema registry URL is required for protobuf serialization")
	}
	if strings.TrimSpace(k.SchemaRegistry.URL) != "" || format == KafkaSerializationProtobuf {
		if err := k.SchemaRegistry.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c KafkaSchemaRegistry) Validate() error {
	parsed, err := url.Parse(strings.TrimSpace(c.URL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("kafka: schema registry URL must be an absolute HTTP(S) URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("kafka: schema registry URL must not contain a query or fragment")
	}
	if (strings.TrimSpace(c.Username) == "") != (c.Password == "") {
		return fmt.Errorf("kafka: schema registry username and password must be provided together")
	}
	if c.RequestTimeout < 0 || c.RequestTimeout > 30*time.Second {
		return fmt.Errorf("kafka: schema registry request timeout must be between 0 and 30s")
	}
	if c.CacheTTL < 0 || c.CacheTTL > 24*time.Hour {
		return fmt.Errorf("kafka: schema registry cache TTL must be between 0 and 24h")
	}
	if c.CacheCapacity < 0 || c.CacheCapacity > 100_000 {
		return fmt.Errorf("kafka: schema registry cache capacity must be between 0 and 100000")
	}
	compatibility := strings.ToUpper(strings.TrimSpace(c.Compatibility))
	if compatibility == "" {
		compatibility = "BACKWARD_TRANSITIVE"
	}
	switch compatibility {
	case "NONE", "BACKWARD", "BACKWARD_TRANSITIVE", "FORWARD", "FORWARD_TRANSITIVE", "FULL", "FULL_TRANSITIVE":
		return nil
	default:
		return fmt.Errorf("kafka: unsupported schema compatibility %q", c.Compatibility)
	}
}

// KafkaProducer makes delivery durability and latency explicit instead of
// depending on kafka-go defaults that may change between library versions.
type KafkaProducer struct {
	MaxAttempts     int           `yaml:"maxAttempts" env:"KAFKA_PRODUCER_MAX_ATTEMPTS" env-default:"10"`
	BatchSize       int           `yaml:"batchSize" env:"KAFKA_PRODUCER_BATCH_SIZE" env-default:"100"`
	BatchBytes      int64         `yaml:"batchBytes" env:"KAFKA_PRODUCER_BATCH_BYTES" env-default:"1048576"`
	BatchTimeout    time.Duration `yaml:"batchTimeout" env:"KAFKA_PRODUCER_BATCH_TIMEOUT" env-default:"10ms"`
	ReadTimeout     time.Duration `yaml:"readTimeout" env:"KAFKA_PRODUCER_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout    time.Duration `yaml:"writeTimeout" env:"KAFKA_PRODUCER_WRITE_TIMEOUT" env-default:"10s"`
	DeliveryTimeout time.Duration `yaml:"deliveryTimeout" env:"KAFKA_PRODUCER_DELIVERY_TIMEOUT" env-default:"30s"`
	Compression     string        `yaml:"compression" env:"KAFKA_PRODUCER_COMPRESSION" env-default:"lz4"`
}

type KafkaTLS struct {
	Enabled    bool   `yaml:"enabled" env:"KAFKA_TLS_ENABLED" env-default:"false"`
	CAFile     string `yaml:"caFile" env:"KAFKA_TLS_CA_FILE"`
	CertFile   string `yaml:"certFile" env:"KAFKA_TLS_CERT_FILE"`
	KeyFile    string `yaml:"keyFile" env:"KAFKA_TLS_KEY_FILE"`
	ServerName string `yaml:"serverName" env:"KAFKA_TLS_SERVER_NAME"`
}

type KafkaSASL struct {
	Mechanism string `yaml:"mechanism" env:"KAFKA_SASL_MECHANISM"`
	Username  string `yaml:"username" env:"KAFKA_SASL_USERNAME"`
	Password  string `yaml:"password" env:"KAFKA_SASL_PASSWORD"`
}

// Outbox controls the polling publisher fallback. Production installations can
// disable the publisher and use CDC, but these defaults are safe for horizontal
// polling workers and drain backlog without an artificial one-second ceiling.
type Outbox struct {
	Enabled         bool          `yaml:"enabled" env:"OUTBOX_PUBLISHER_ENABLED" env-default:"true"`
	PollInterval    time.Duration `yaml:"pollInterval" env:"OUTBOX_POLL_INTERVAL" env-default:"100ms"`
	MetricsInterval time.Duration `yaml:"metricsInterval" env:"OUTBOX_METRICS_INTERVAL" env-default:"15s"`
	BatchSize       int           `yaml:"batchSize" env:"OUTBOX_BATCH_SIZE" env-default:"500"`
	Parallelism     int           `yaml:"parallelism" env:"OUTBOX_PARALLELISM" env-default:"16"`
	StaleAfter      time.Duration `yaml:"staleAfter" env:"OUTBOX_STALE_AFTER" env-default:"2m"`
	MaxAttempts     int           `yaml:"maxAttempts" env:"OUTBOX_MAX_ATTEMPTS" env-default:"20"`
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
