package telemetry

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func SetUpOtelSdk(config *config.Otel, ctx context.Context) (shutdown func(ctx context.Context) error, err error) {
	var shutdownFuncs []func(ctx context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error

		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set up propagators.
	prop := newPropagators()

	otel.SetTextMapPropagator(prop)

	// Set up trace provider

	traceProv, err := newTraceProvider(config.Endpoint, config.Service)

	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, traceProv.Shutdown)
	otel.SetTracerProvider(traceProv)

	// Set up meter provider
	metricsEndpoint := strings.TrimSpace(config.MetricsEndpoint)
	if metricsEndpoint == "" {
		metricsEndpoint = config.Endpoint
	}
	meterProv, err := newMeterProvider(metricsEndpoint, config.Service, config.MetricsExportInterval)

	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProv.Shutdown)
	otel.SetMeterProvider(meterProv)

	// Set up logger provider
	loggerProvider, err := newLoggerProvider()

	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	return
}

func newPropagators() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

func newTraceProvider(endpoint string, service string) (*trace.TracerProvider, error) {
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			traceHTTPClientOptions(endpoint)...,
		),
	)

	if err != nil {
		return nil, err
	}

	rs, err := newResource(service)

	if err != nil {
		return nil, err
	}

	tracerprovider := trace.NewTracerProvider(
		trace.WithBatcher(
			exporter,
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
			trace.WithBatchTimeout(trace.DefaultScheduleDelay*time.Millisecond),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
		trace.WithResource(rs),
	)
	return tracerprovider, nil

}

type traceEndpointConfig struct {
	endpoint    string
	endpointURL string
	insecure    bool
}

func traceHTTPClientOptions(endpoint string) []otlptracehttp.Option {
	cfg := newTraceEndpointConfig(endpoint)
	opts := make([]otlptracehttp.Option, 0, 2)
	if cfg.endpointURL != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(cfg.endpointURL))
	} else {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.endpoint))
	}
	if cfg.insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	return opts
}

func newTraceEndpointConfig(endpoint string) traceEndpointConfig {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		if parsed.Path != "" && parsed.Path != "/" {
			return traceEndpointConfig{endpointURL: endpoint}
		}
		return traceEndpointConfig{
			endpoint: parsed.Host,
			insecure: parsed.Scheme == "http",
		}
	}

	return traceEndpointConfig{
		endpoint: endpoint,
		insecure: true,
	}
}

func newMeterProvider(endpoint, service string, interval time.Duration) (*metric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(context.Background(), metricHTTPClientOptions(endpoint)...)
	if err != nil {
		return nil, err
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	rs, err := newResource(service)
	if err != nil {
		return nil, err
	}
	reader := metric.NewPeriodicReader(exporter, metric.WithInterval(interval))
	return metric.NewMeterProvider(metric.WithReader(reader), metric.WithResource(rs)), nil
}

func metricHTTPClientOptions(endpoint string) []otlpmetrichttp.Option {
	cfg := newTraceEndpointConfig(endpoint)
	opts := make([]otlpmetrichttp.Option, 0, 2)
	if cfg.endpointURL != "" {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(cfg.endpointURL))
	} else {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.endpoint))
	}
	if cfg.insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	return opts
}

func newResource(service string) (*resource.Resource, error) {
	return resource.New(
		context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(service)),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOSType(),
		resource.WithProcessRuntimeDescription(),
	)
}

func newLoggerProvider() (*log.LoggerProvider, error) {
	exporter, err := stdoutlog.New(stdoutlog.WithPrettyPrint())

	if err != nil {
		return nil, err
	}

	loggerProvider := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(exporter)))
	return loggerProvider, nil
}
