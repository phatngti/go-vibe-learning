package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/payment.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderEnvironmentOverridesLocalSimulator(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/payment.yaml")
	t.Setenv("PAYMENT_PROVIDER", "stripe")
	cfg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider.Kind != "stripe" {
		t.Fatalf("provider kind = %q, want stripe", cfg.Provider.Kind)
	}
}

func TestRawCardCheckoutCannotBeEnabledInProduction(t *testing.T) {
	cfg := new(Config)
	cfg.LegacyCheckout.RawCardEnabled = true
	cfg.Logger.Mode = "production"
	if cfg.RawCardEnabled() {
		t.Fatal("RawCardEnabled() = true in production")
	}
	cfg.Logger.Mode = "development"
	if !cfg.RawCardEnabled() {
		t.Fatal("RawCardEnabled() = false for explicitly enabled local development")
	}
}
