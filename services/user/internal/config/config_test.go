package config

import (
	"testing"
	"time"
)

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/user.yaml")
	cfg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SyncJobs.PageSize != 100 || cfg.SyncJobs.LeaseDuration != time.Minute {
		t.Fatalf("sync job config = %+v", cfg.SyncJobs)
	}
}

func TestSyncJobConfigRejectsLeaseShorterThanProviderTimeout(t *testing.T) {
	cfg := &Config{}
	cfg.SyncJobs.PageSize = 100
	cfg.SyncJobs.Parallelism = 1
	cfg.SyncJobs.PollInterval = time.Second
	cfg.SyncJobs.ProviderTimeout = time.Minute
	cfg.SyncJobs.LeaseDuration = time.Minute
	cfg.SyncJobs.MaxAttempts = 3
	cfg.SyncJobs.RetryBaseDelay = time.Second
	cfg.SyncJobs.RetryMaxDelay = time.Minute
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid lease error")
	}
}
