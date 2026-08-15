package config

import (
	"strings"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"go.uber.org/fx"
)

type Config struct {
	Postgres       platformconfig.Postgres        `yaml:"postgres"`
	Logger         platformconfig.Logger          `yaml:"logger"`
	Server         platformconfig.Server          `yaml:"server"`
	Gateway        platformconfig.Gateway         `yaml:"gateway"`
	Otel           platformconfig.Otel            `yaml:"otel"`
	Kafka          platformconfig.Kafka           `yaml:"kafka"`
	Outbox         platformconfig.Outbox          `yaml:"outbox"`
	Authentication gatewayauth.VerificationConfig `yaml:"authentication"`
	Initiation     struct {
		RequireIdempotency bool `yaml:"requireIdempotency" env:"PAYMENT_REQUIRE_INITIATE_IDEMPOTENCY" env-default:"false"`
	} `yaml:"initiation"`
	Reconciliation struct {
		Enabled    bool          `yaml:"enabled" env:"PAYMENT_RECONCILIATION_ENABLED" env-default:"true"`
		Interval   time.Duration `yaml:"interval" env:"PAYMENT_RECONCILIATION_INTERVAL" env-default:"15s"`
		StaleAfter time.Duration `yaml:"staleAfter" env:"PAYMENT_RECONCILIATION_STALE_AFTER" env-default:"2m"`
		BatchSize  int           `yaml:"batchSize" env:"PAYMENT_RECONCILIATION_BATCH_SIZE" env-default:"100"`
	} `yaml:"reconciliation"`
	Provider struct {
		Kind   string `yaml:"kind" env:"PAYMENT_PROVIDER" env-default:"simulator"`
		Stripe struct {
			SecretKey        string        `yaml:"secretKey" env:"STRIPE_SECRET_KEY"`
			WebhookSecrets   []string      `yaml:"webhookSecrets" env:"STRIPE_WEBHOOK_SECRETS" env-separator:","`
			BaseURL          string        `yaml:"baseUrl" env:"STRIPE_BASE_URL" env-default:"https://api.stripe.com"`
			HTTPTimeout      time.Duration `yaml:"httpTimeout" env:"STRIPE_HTTP_TIMEOUT" env-default:"10s"`
			WebhookTolerance time.Duration `yaml:"webhookTolerance" env:"STRIPE_WEBHOOK_TOLERANCE" env-default:"5m"`
		} `yaml:"stripe"`
	} `yaml:"paymentProvider"`
	LegacyCheckout struct {
		RawCardEnabled bool `yaml:"rawCardEnabled" env:"PAYMENT_LEGACY_RAW_CARD_ENABLED" env-default:"false"`
	} `yaml:"legacyCheckout"`
}

// RawCardEnabled keeps the v1 simulator checkout available for local learning
// while making it impossible to enable in a production logger profile.
func (c *Config) RawCardEnabled() bool {
	if c == nil || strings.EqualFold(strings.TrimSpace(c.Logger.Mode), "production") {
		return false
	}
	return c.LegacyCheckout.RawCardEnabled
}

func New() (*Config, error) {
	var cfg Config
	return &cfg, platformconfig.Load("payment", &cfg)
}

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

var Module = fx.Module("payment-config", fx.Provide(New, Components))
