package kafka

import (
	"testing"

	platformconfig "github/master_go_cqrs/internal/platform/config"
)

func TestKafkaSecurityRejectsCredentialsWithoutTLS(t *testing.T) {
	_, _, err := kafkaSecurity(&platformconfig.Kafka{
		SASL: platformconfig.KafkaSASL{Mechanism: "scram-sha-512", Username: "service", Password: "secret"},
	})
	if err == nil {
		t.Fatal("kafkaSecurity() error = nil")
	}
}

func TestKafkaSecurityBuildsTLSAndSCRAM(t *testing.T) {
	tlsConfig, mechanism, err := kafkaSecurity(&platformconfig.Kafka{
		TLS:  platformconfig.KafkaTLS{Enabled: true, ServerName: "broker.internal"},
		SASL: platformconfig.KafkaSASL{Mechanism: "scram-sha-256", Username: "service", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tlsConfig == nil || tlsConfig.ServerName != "broker.internal" || mechanism == nil {
		t.Fatalf("TLS=%+v mechanism=%+v", tlsConfig, mechanism)
	}
}
