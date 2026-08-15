package payment

import (
	"testing"

	"github/master_go_cqrs/services/payment/internal/config"
)

func TestPaymentProviderFactoryRejectsSimulatorInProduction(t *testing.T) {
	cfg := new(config.Config)
	cfg.Logger.Mode = "production"
	cfg.Provider.Kind = "simulator"
	if _, err := newPaymentProvider(cfg); err == nil {
		t.Fatal("newPaymentProvider() error = nil for production simulator")
	}
}

func TestPaymentProviderFactoryAllowsSimulatorForLocalDevelopment(t *testing.T) {
	cfg := new(config.Config)
	cfg.Logger.Mode = "development"
	cfg.Provider.Kind = "simulator"
	provider, err := newPaymentProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name() != "simulator" {
		t.Fatalf("provider name = %q", provider.Name())
	}
}
