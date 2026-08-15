package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/orchestrator.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateOrchestratorDerivesImmutableOwner(t *testing.T) {
	cfg := &Config{}
	cfg.Orchestrator.Engine = OrchestratorEngineSaga
	if err := cfg.ValidateOrchestrator(); err != nil {
		t.Fatal(err)
	}
	if cfg.Orchestrator.Owner != "saga:v1" || cfg.Orchestrator.Generation != 1 {
		t.Fatalf("identity = %q generation %d", cfg.Orchestrator.Owner, cfg.Orchestrator.Generation)
	}
}

func TestValidateOrchestratorRejectsOwnerGenerationMismatch(t *testing.T) {
	cfg := &Config{}
	cfg.Orchestrator.Engine = OrchestratorEngineSaga
	cfg.Orchestrator.Owner = "temporal:v2"
	cfg.Orchestrator.Generation = 2
	if err := cfg.ValidateOrchestrator(); err == nil {
		t.Fatal("ValidateOrchestrator() error = nil")
	}
}

func TestConsumerGroupsSeparateEnginesAndGenerations(t *testing.T) {
	saga := &Config{}
	saga.Orchestrator.Engine = OrchestratorEngineSaga
	saga.Orchestrator.Generation = 1
	temporal := &Config{}
	temporal.Orchestrator.Engine = OrchestratorEngineTemporal
	temporal.Orchestrator.Generation = 2

	if got := saga.ConsumerGroupID("payments"); got != "orchestrator-payments" {
		t.Fatalf("Saga v1 group = %q", got)
	}
	if got := temporal.ConsumerGroupID("payments"); got != "orchestrator-temporal-v2-payments" {
		t.Fatalf("Temporal v2 group = %q", got)
	}
	if saga.ConsumerGroupID("payments") == temporal.ConsumerGroupID("payments") {
		t.Fatal("Saga and Temporal consumer groups overlap")
	}
}
