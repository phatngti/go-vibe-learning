package architecture

import (
	"testing"

	"github/master_go_cqrs/services/affiliate"
	"github/master_go_cqrs/services/agent"
	"github/master_go_cqrs/services/payment"
	"github/master_go_cqrs/services/settlement"
	"github/master_go_cqrs/services/user"

	"go.uber.org/fx"
)

func TestServiceRuntimeDependencyGraphs(t *testing.T) {
	tests := map[string]fx.Option{
		"user":       user.Options(),
		"payment":    payment.Options(),
		"affiliate":  affiliate.Options(),
		"settlement": settlement.Options(),
		"agent":      agent.Options(),
	}
	for name, option := range tests {
		t.Run(name, func(t *testing.T) {
			if err := fx.ValidateApp(option); err != nil {
				t.Fatalf("invalid Fx dependency graph: %v", err)
			}
		})
	}
}
