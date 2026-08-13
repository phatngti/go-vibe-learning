package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/orchestrator.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}
