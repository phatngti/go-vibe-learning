package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	platformconfig "github/master_go_cqrs/internal/platform/config"

	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func kafkaSecurity(cfg *platformconfig.Kafka) (*tls.Config, sasl.Mechanism, error) {
	tlsConfig, err := kafkaTLSConfig(cfg.TLS)
	if err != nil {
		return nil, nil, err
	}
	mechanism, err := kafkaSASLMechanism(cfg.SASL)
	if err != nil {
		return nil, nil, err
	}
	if mechanism != nil && tlsConfig == nil {
		return nil, nil, fmt.Errorf("kafka: SASL credentials require TLS")
	}
	return tlsConfig, mechanism, nil
}

func kafkaTLSConfig(settings platformconfig.KafkaTLS) (*tls.Config, error) {
	if !settings.Enabled {
		return nil, nil
	}
	if (settings.CertFile == "") != (settings.KeyFile == "") {
		return nil, fmt.Errorf("kafka: TLS certificate and key must be configured together")
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: settings.ServerName}
	if settings.CAFile != "" {
		pem, err := os.ReadFile(settings.CAFile)
		if err != nil {
			return nil, fmt.Errorf("kafka: read CA certificate: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("kafka: parse CA certificate %s", settings.CAFile)
		}
		config.RootCAs = roots
	}
	if settings.CertFile != "" {
		certificate, err := tls.LoadX509KeyPair(settings.CertFile, settings.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("kafka: load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}

func kafkaSASLMechanism(settings platformconfig.KafkaSASL) (sasl.Mechanism, error) {
	mechanism := strings.ToLower(strings.TrimSpace(settings.Mechanism))
	if mechanism == "" {
		return nil, nil
	}
	if settings.Username == "" || settings.Password == "" {
		return nil, fmt.Errorf("kafka: SASL username and password are required")
	}
	switch mechanism {
	case "plain":
		return plain.Mechanism{Username: settings.Username, Password: settings.Password}, nil
	case "scram-sha-256":
		return scram.Mechanism(scram.SHA256, settings.Username, settings.Password)
	case "scram-sha-512":
		return scram.Mechanism(scram.SHA512, settings.Username, settings.Password)
	default:
		return nil, fmt.Errorf("kafka: unsupported SASL mechanism %q", settings.Mechanism)
	}
}
