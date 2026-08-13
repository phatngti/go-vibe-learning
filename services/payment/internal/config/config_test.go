package config

import "testing"

func TestOwnedConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../config/payment.yaml")
	if _, err := New(); err != nil {
		t.Fatal(err)
	}
}
