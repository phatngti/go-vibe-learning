package orchestrator

import (
	"testing"

	"github/master_go_cqrs/internal/config"

	"go.uber.org/fx"
)

func TestEngineModuleGraphs(t *testing.T) {
	for _, engine := range []string{config.OrchestratorEngineSaga, config.OrchestratorEngineTemporal} {
		t.Run(engine, func(t *testing.T) {
			cfg := testConfig(engine)
			options, err := Options(cfg)
			if err != nil {
				t.Fatalf("Options() error = %v", err)
			}
			if err := fx.ValidateApp(options); err != nil {
				t.Fatalf("fx.ValidateApp() error = %v", err)
			}
		})
	}
}

func TestOptionsRejectsUnknownEngine(t *testing.T) {
	cfg := testConfig("other")
	if _, err := Options(cfg); err == nil {
		t.Fatal("Options() error = nil, want invalid engine error")
	}
}

func testConfig(engine string) *config.Config {
	cfg := &config.Config{}
	cfg.Orchestrator.Engine = engine
	cfg.Orchestrator.Temporal.Address = "temporal:7233"
	cfg.Orchestrator.Temporal.Namespace = "master-go-cqrs"
	cfg.Orchestrator.Temporal.TaskQueue = "payment-orchestration"
	return cfg
}
