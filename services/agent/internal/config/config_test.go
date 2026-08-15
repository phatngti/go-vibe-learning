package config

import (
	"strings"
	"testing"
	"time"
)

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/agent.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}

func TestProductionForcesHTTPSAndVerifiedCallbacks(t *testing.T) {
	cfg := &Config{}
	cfg.Logger.Mode = "production"
	cfg.Callback.SigningSecret = strings.Repeat("s", 32)
	cfg.Callback.SigningKeyVersion = "key-1"
	cfg.Callback.VerifiedDomains = []string{"merchant.example"}
	cfg.Callback.RequestTimeout = 10 * time.Second
	cfg.Callback.DialTimeout = 10 * time.Second
	cfg.Callback.TLSHandshakeTimeout = 10 * time.Second
	cfg.Callback.ResponseHeaderTimeout = 10 * time.Second

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Callback.RequireHTTPS || !cfg.Callback.RequireVerified {
		t.Fatalf("production callback policy = %+v", cfg.Callback)
	}
}

func TestProductionRejectsMissingSigningSecret(t *testing.T) {
	cfg := &Config{}
	cfg.Logger.Mode = "production"
	cfg.Callback.SigningKeyVersion = "key-1"
	cfg.Callback.VerifiedDomains = []string{"merchant.example"}
	cfg.Callback.RequestTimeout = 10 * time.Second
	cfg.Callback.DialTimeout = 10 * time.Second
	cfg.Callback.TLSHandshakeTimeout = 10 * time.Second
	cfg.Callback.ResponseHeaderTimeout = 10 * time.Second

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing signing secret error")
	}
}
