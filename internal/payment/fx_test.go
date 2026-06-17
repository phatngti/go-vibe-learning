package payment

import (
	"testing"

	"go.uber.org/fx"
)

func TestModuleGraph(t *testing.T) {
	if err := fx.ValidateApp(Options()); err != nil {
		t.Fatalf("fx.ValidateApp() error = %v", err)
	}
}
