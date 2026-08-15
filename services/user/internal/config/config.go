package config

import (
	"fmt"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/security/gatewayauth"
	"go.uber.org/fx"
)

type Config struct {
	Postgres       platformconfig.Postgres        `yaml:"postgres"`
	Logger         platformconfig.Logger          `yaml:"logger"`
	Server         platformconfig.Server          `yaml:"server"`
	Keycloak       platformconfig.Keycloak        `yaml:"keycloak"`
	Otel           platformconfig.Otel            `yaml:"otel"`
	Kafka          platformconfig.Kafka           `yaml:"kafka"`
	Outbox         platformconfig.Outbox          `yaml:"outbox"`
	SyncJobs       SyncJobs                       `yaml:"syncJobs"`
	Authentication gatewayauth.VerificationConfig `yaml:"authentication"`
}

type SyncJobs struct {
	Enabled         bool          `yaml:"enabled" env:"USER_SYNC_JOBS_ENABLED" env-default:"true"`
	PageSize        int           `yaml:"pageSize" env:"USER_SYNC_JOBS_PAGE_SIZE" env-default:"100"`
	Parallelism     int           `yaml:"parallelism" env:"USER_SYNC_JOBS_PARALLELISM" env-default:"4"`
	PollInterval    time.Duration `yaml:"pollInterval" env:"USER_SYNC_JOBS_POLL_INTERVAL" env-default:"250ms"`
	LeaseDuration   time.Duration `yaml:"leaseDuration" env:"USER_SYNC_JOBS_LEASE_DURATION" env-default:"1m"`
	ProviderTimeout time.Duration `yaml:"providerTimeout" env:"USER_SYNC_PROVIDER_TIMEOUT" env-default:"15s"`
	MaxAttempts     int           `yaml:"maxAttempts" env:"USER_SYNC_JOBS_MAX_ATTEMPTS" env-default:"12"`
	RetryBaseDelay  time.Duration `yaml:"retryBaseDelay" env:"USER_SYNC_JOBS_RETRY_BASE_DELAY" env-default:"1s"`
	RetryMaxDelay   time.Duration `yaml:"retryMaxDelay" env:"USER_SYNC_JOBS_RETRY_MAX_DELAY" env-default:"5m"`
}

func New() (*Config, error) {
	var cfg Config
	if err := platformconfig.Load("user", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.SyncJobs.PageSize <= 0 || c.SyncJobs.PageSize > 1000 {
		return fmt.Errorf("user sync job page size must be between 1 and 1000")
	}
	if c.SyncJobs.Parallelism <= 0 || c.SyncJobs.Parallelism > 64 {
		return fmt.Errorf("user sync job parallelism must be between 1 and 64")
	}
	if c.SyncJobs.PollInterval <= 0 || c.SyncJobs.ProviderTimeout <= 0 {
		return fmt.Errorf("user sync job poll interval and provider timeout must be positive")
	}
	if c.SyncJobs.LeaseDuration <= c.SyncJobs.ProviderTimeout {
		return fmt.Errorf("user sync job lease duration must exceed provider timeout")
	}
	if c.SyncJobs.MaxAttempts <= 0 {
		return fmt.Errorf("user sync job max attempts must be positive")
	}
	if c.SyncJobs.RetryBaseDelay <= 0 || c.SyncJobs.RetryMaxDelay < c.SyncJobs.RetryBaseDelay {
		return fmt.Errorf("user sync job retry delays are invalid")
	}
	return nil
}

type PlatformComponents struct {
	fx.Out
	Postgres       *platformconfig.Postgres
	Logger         *platformconfig.Logger
	Server         *platformconfig.Server
	Keycloak       *platformconfig.Keycloak
	Otel           *platformconfig.Otel
	Kafka          *platformconfig.Kafka
	Outbox         *platformconfig.Outbox
	Authentication *gatewayauth.VerificationConfig
}

func Components(cfg *Config) PlatformComponents {
	return PlatformComponents{
		Postgres: &cfg.Postgres, Logger: &cfg.Logger, Server: &cfg.Server,
		Keycloak: &cfg.Keycloak, Otel: &cfg.Otel, Kafka: &cfg.Kafka, Outbox: &cfg.Outbox,
		Authentication: &cfg.Authentication,
	}
}

var Module = fx.Module("user-config", fx.Provide(New, Components))
