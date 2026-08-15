package temporal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github/master_go_cqrs/internal/platform/observability/logger"
	"github/master_go_cqrs/services/orchestrator/internal/config"

	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func NewClient(lifecycle fx.Lifecycle, cfg *config.Config, l logger.Factory) (client.Client, error) {
	settings := cfg.Orchestrator.Temporal
	l.Bg().Info("Connecting to Temporal", zap.String("address", settings.Address), zap.String("namespace", settings.Namespace))
	tlsConfig, err := temporalTLSConfig(settings.TLSEnabled, settings.TLSCAFile, settings.TLSCertFile, settings.TLSKeyFile, settings.TLSServerName)
	if err != nil {
		return nil, err
	}
	tracingInterceptor, err := temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
	if err != nil {
		return nil, fmt.Errorf("configure Temporal tracing: %w", err)
	}
	metricsHandler := temporalotel.NewMetricsHandler(temporalotel.MetricsHandlerOptions{
		OnError: func(err error) {
			l.Bg().Error("Temporal metrics instrumentation failed", zap.Error(err))
		},
	})
	c, err := client.Dial(client.Options{
		HostPort: settings.Address, Namespace: settings.Namespace,
		ConnectionOptions: client.ConnectionOptions{TLS: tlsConfig},
		MetricsHandler:    metricsHandler,
		Interceptors:      []interceptor.ClientInterceptor{tracingInterceptor},
	})
	if err != nil {
		return nil, fmt.Errorf("connect to Temporal: %w", err)
	}
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		c.Close()
		return nil
	}})
	return c, nil
}

func temporalTLSConfig(enabled bool, caFile, certFile, keyFile, serverName string) (*tls.Config, error) {
	if !enabled {
		return nil, nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read Temporal CA certificate: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse Temporal CA certificate %s", caFile)
		}
		config.RootCAs = roots
	}
	if certFile != "" || keyFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load Temporal client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}
