package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/agent.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}
