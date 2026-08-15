package config

import (
	"fmt"
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
	Otel           platformconfig.Otel            `yaml:"otel"`
	Kafka          platformconfig.Kafka           `yaml:"kafka"`
	Outbox         platformconfig.Outbox          `yaml:"outbox"`
	Authentication gatewayauth.VerificationConfig `yaml:"authentication"`
	Callback       Callback                       `yaml:"callback"`
}

type Callback struct {
	RequireHTTPS          bool          `yaml:"requireHttps" env:"CALLBACK_REQUIRE_HTTPS" env-default:"false"`
	RequireVerified       bool          `yaml:"requireVerified" env:"CALLBACK_REQUIRE_VERIFIED" env-default:"false"`
	VerifiedDomains       []string      `yaml:"verifiedDomains" env:"CALLBACK_VERIFIED_DOMAINS" env-separator:","`
	SigningKeyVersion     string        `yaml:"signingKeyVersion" env:"CALLBACK_SIGNING_KEY_VERSION" env-default:"local-v1"`
	SigningSecret         string        `yaml:"signingSecret" env:"CALLBACK_SIGNING_SECRET"`
	RequestTimeout        time.Duration `yaml:"requestTimeout" env:"CALLBACK_REQUEST_TIMEOUT" env-default:"10s"`
	DialTimeout           time.Duration `yaml:"dialTimeout" env:"CALLBACK_DIAL_TIMEOUT" env-default:"3s"`
	TLSHandshakeTimeout   time.Duration `yaml:"tlsHandshakeTimeout" env:"CALLBACK_TLS_HANDSHAKE_TIMEOUT" env-default:"5s"`
	ResponseHeaderTimeout time.Duration `yaml:"responseHeaderTimeout" env:"CALLBACK_RESPONSE_HEADER_TIMEOUT" env-default:"5s"`
	ChallengeTTL          time.Duration `yaml:"challengeTtl" env:"CALLBACK_CHALLENGE_TTL" env-default:"24h"`
	WorkerEnabled         bool          `yaml:"workerEnabled" env:"CALLBACK_WORKER_ENABLED" env-default:"true"`
	WorkerParallelism     int           `yaml:"workerParallelism" env:"CALLBACK_WORKER_PARALLELISM" env-default:"4"`
	WorkerPollInterval    time.Duration `yaml:"workerPollInterval" env:"CALLBACK_WORKER_POLL_INTERVAL" env-default:"1s"`
	WorkerLeaseDuration   time.Duration `yaml:"workerLeaseDuration" env:"CALLBACK_WORKER_LEASE_DURATION" env-default:"30s"`
}

func New() (*Config, error) {
	var cfg Config
	if err := platformconfig.Load("agent", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Callback.ChallengeTTL == 0 {
		c.Callback.ChallengeTTL = 24 * time.Hour
	}
	if c.Callback.WorkerParallelism == 0 {
		c.Callback.WorkerParallelism = 4
	}
	if c.Callback.WorkerPollInterval == 0 {
		c.Callback.WorkerPollInterval = time.Second
	}
	if c.Callback.WorkerLeaseDuration == 0 {
		c.Callback.WorkerLeaseDuration = 30 * time.Second
	}
	production := strings.EqualFold(strings.TrimSpace(c.Logger.Mode), "production")
	if production {
		c.Callback.RequireHTTPS = true
		c.Callback.RequireVerified = true
	}
	if c.Callback.RequestTimeout <= 0 || c.Callback.DialTimeout <= 0 || c.Callback.TLSHandshakeTimeout <= 0 || c.Callback.ResponseHeaderTimeout <= 0 {
		return fmt.Errorf("callback timeouts must be positive")
	}
	if c.Callback.ChallengeTTL <= 0 {
		return fmt.Errorf("callback challenge TTL must be positive")
	}
	if c.Callback.WorkerParallelism <= 0 || c.Callback.WorkerParallelism > 64 {
		return fmt.Errorf("callback worker parallelism must be between 1 and 64")
	}
	if c.Callback.WorkerPollInterval <= 0 || c.Callback.WorkerLeaseDuration <= c.Callback.RequestTimeout {
		return fmt.Errorf("callback worker timing is invalid: lease duration must exceed request timeout")
	}
	if strings.TrimSpace(c.Callback.SigningSecret) != "" && len(c.Callback.SigningSecret) < 32 {
		return fmt.Errorf("callback signing secret must contain at least 32 bytes")
	}
	if production {
		if strings.TrimSpace(c.Callback.SigningSecret) == "" || strings.TrimSpace(c.Callback.SigningKeyVersion) == "" {
			return fmt.Errorf("callback signing secret and key version are required in production")
		}
		if len(c.Callback.VerifiedDomains) == 0 {
			return fmt.Errorf("at least one verified callback domain is required in production")
		}
	}
	return nil
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

var Module = fx.Module("agent-config", fx.Provide(New, Components))
