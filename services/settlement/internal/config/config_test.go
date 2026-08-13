package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/settlement.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}
