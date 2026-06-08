package app

import (
	"testing"

	"go.uber.org/fx"
)

func TestModuleGraph(t *testing.T) {
	err := fx.ValidateApp(Options())
	if err != nil {
		t.Fatalf("fx.ValidateApp() error = %v", err)
	}
}
