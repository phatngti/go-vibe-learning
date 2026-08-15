package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestLoadAppliesNestedProductionDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.yaml")
	if err := os.WriteFile(path, []byte("kafka:\n  brokers: [localhost:9092]\noutbox:\n  enabled: true\notel:\n  service: test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	var target struct {
		Kafka  Kafka  `yaml:"kafka"`
		Outbox Outbox `yaml:"outbox"`
		Otel   Otel   `yaml:"otel"`
	}
	if err := Load("payment", &target); err != nil {
		t.Fatal(err)
	}
	if target.Kafka.Producer.DeliveryTimeout != 30*time.Second || target.Kafka.Producer.Compression != "lz4" {
		t.Fatalf("producer defaults = %+v", target.Kafka.Producer)
	}
	if target.Outbox.MetricsInterval != 15*time.Second || target.Otel.MetricsExportInterval != 15*time.Second {
		t.Fatalf("metrics defaults: outbox=%s otel=%s", target.Outbox.MetricsInterval, target.Otel.MetricsExportInterval)
	}
	if target.Kafka.Serialization.Format != KafkaSerializationJSON {
		t.Fatalf("serialization default = %q, want %q", target.Kafka.Serialization.Format, KafkaSerializationJSON)
	}
	if target.Kafka.SchemaRegistry.Compatibility != "BACKWARD_TRANSITIVE" || target.Kafka.SchemaRegistry.RequestTimeout != 5*time.Second {
		t.Fatalf("schema registry defaults = %+v", target.Kafka.SchemaRegistry)
	}
}

func TestKafkaValidateSerialization(t *testing.T) {
	tests := []struct {
		name    string
		config  Kafka
		wantErr string
	}{
		{name: "legacy JSON remains valid", config: Kafka{}},
		{
			name: "protobuf registry is valid",
			config: Kafka{
				Serialization:  KafkaSerialization{Format: KafkaSerializationProtobuf},
				SchemaRegistry: KafkaSchemaRegistry{URL: "https://registry.example.com", Compatibility: "BACKWARD_TRANSITIVE"},
			},
		},
		{
			name:    "protobuf requires registry",
			config:  Kafka{Serialization: KafkaSerialization{Format: KafkaSerializationProtobuf}},
			wantErr: "URL is required",
		},
		{
			name:    "format is explicit",
			config:  Kafka{Serialization: KafkaSerialization{Format: "avro"}},
			wantErr: "unsupported serialization format",
		},
		{
			name: "timeout is bounded",
			config: Kafka{
				Serialization:  KafkaSerialization{Format: KafkaSerializationProtobuf},
				SchemaRegistry: KafkaSchemaRegistry{URL: "https://registry.example.com", RequestTimeout: 31 * time.Second},
			},
			wantErr: "request timeout",
		},
		{
			name: "credentials are complete",
			config: Kafka{
				Serialization:  KafkaSerialization{Format: KafkaSerializationProtobuf},
				SchemaRegistry: KafkaSchemaRegistry{URL: "https://registry.example.com", Username: "service"},
			},
			wantErr: "username and password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateSerialization()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("ValidateSerialization() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("ValidateSerialization() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
