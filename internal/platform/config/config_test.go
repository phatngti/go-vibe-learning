package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsConfigAndEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8081\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	t.Setenv("SERVER_PORT", "9091")
	var target struct {
		Server Server `yaml:"server"`
	}
	if err := Load("payment", &target); err != nil {
		t.Fatal(err)
	}
	if target.Server.Port != 9091 {
		t.Fatalf("port = %d, want 9091", target.Server.Port)
	}
}
