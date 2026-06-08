package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPathPrefersYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), `server:
  port: 9000
`)
	writeConfig(t, filepath.Join(dir, "config.yml"), `server:
  port: 8000
`)
	chdir(t, dir)
	t.Setenv("SERVICE_NAME", "")

	got, err := defaultConfigPath()
	if err != nil {
		t.Fatalf("defaultConfigPath() error = %v", err)
	}

	want := filepath.Join(currentDir(t), "config.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathFallsBackToYML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yml"), `server:
  port: 8000
`)
	chdir(t, dir)
	t.Setenv("SERVICE_NAME", "")

	got, err := defaultConfigPath()
	if err != nil {
		t.Fatalf("defaultConfigPath() error = %v", err)
	}

	want := filepath.Join(currentDir(t), "config.yml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathPrefersServiceConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), `server:
  port: 9000
`)
	writeConfig(t, filepath.Join(dir, "config", "payment.yaml"), `server:
  port: 8081
`)
	chdir(t, dir)
	t.Setenv("SERVICE_NAME", "payment")

	got, err := defaultConfigPath()
	if err != nil {
		t.Fatalf("defaultConfigPath() error = %v", err)
	}

	want := filepath.Join(currentDir(t), "config", "payment.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}

func TestNewConfigReadsDefaultYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), `postgres:
  host: "db"
  port: "5433"
  user: "app"
  password: "secret"
  dbname: "app"
logger:
  logLevel: "warn"
  mode: "production"
  encoding: "json"
server:
  port: 9090
keycloak:
  endpoint: "http://keycloak"
  clientId: "client"
  clientSecret: "secret"
  publicKey: ""
  customHeaderName: "Authorization"
  realm: "backend"
otel:
  endpoint: "otel:4318"
  service: "test-service"
`)
	chdir(t, dir)
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("SERVICE_NAME", "")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.Postgres.Host != "db" {
		t.Fatalf("Postgres.Host = %q, want %q", cfg.Postgres.Host, "db")
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Otel.Service != "test-service" {
		t.Fatalf("Otel.Service = %q, want %q", cfg.Otel.Service, "test-service")
	}
}

func TestNewConfigReadsServiceConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config", "agent.yaml"), `postgres:
  host: "db"
  port: "5433"
  user: "app"
  password: "secret"
  dbname: "app"
logger:
  logLevel: "warn"
  mode: "production"
  encoding: "json"
server:
  port: 8084
keycloak:
  endpoint: "http://keycloak"
  clientId: "client"
  clientSecret: "secret"
  publicKey: ""
  customHeaderName: "Authorization"
  realm: "backend"
otel:
  endpoint: "otel:4318"
  service: "agent"
`)
	chdir(t, dir)
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("SERVICE_NAME", "agent")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.Server.Port != 8084 {
		t.Fatalf("Server.Port = %d, want %d", cfg.Server.Port, 8084)
	}
	if cfg.Otel.Service != "agent" {
		t.Fatalf("Otel.Service = %q, want %q", cfg.Otel.Service, "agent")
	}
}

func TestNewConfigReadsOtelEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), `server:
  port: 9090
otel:
  endpoint: "file-otel:4318"
  service: "file-service"
`)
	chdir(t, dir)
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_SERVICE_NAME", "env-service")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.Otel.Endpoint != "http://localhost:4318" {
		t.Fatalf("Otel.Endpoint = %q, want %q", cfg.Otel.Endpoint, "http://localhost:4318")
	}
	if cfg.Otel.Service != "env-service" {
		t.Fatalf("Otel.Service = %q, want %q", cfg.Otel.Service, "env-service")
	}
}

func TestNewConfigPrefersTraceSpecificOtelEndpoint(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), `server:
  port: 9090
otel:
  endpoint: "file-otel:4318"
  service: "file-service"
`)
	chdir(t, dir)
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:4318/v1/traces")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.Otel.Endpoint != "http://localhost:4318/v1/traces" {
		t.Fatalf("Otel.Endpoint = %q, want %q", cfg.Otel.Endpoint, "http://localhost:4318/v1/traces")
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatalf("restore cwd %q error = %v", prev, err)
		}
	})
}

func writeConfig(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func currentDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	return dir
}
