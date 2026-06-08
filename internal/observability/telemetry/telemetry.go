package telemetry

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github/master_go_cqrs/internal/config"

	"go.opentelemetry.io/otel"
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

func SetUpOtelSdk(config *config.Config, ctx context.Context) (shutdown func(ctx context.Context) error, err error) {
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

	traceProv, err := newTraceProvider(config.Otel.Endpoint, config.Otel.Service)

	if err != nil {
		handleErr(err)
		return
	}

	shutdownFuncs = append(shutdownFuncs, traceProv.Shutdown)
	otel.SetTracerProvider(traceProv)

	// Set up meter provider
	meterProv, err := newMeterProvider()

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

	rs, err := resource.New(
		context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(service)),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOSType(),
		resource.WithProcessRuntimeDescription(),
	)

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

func newMeterProvider() (*metric.MeterProvider, error) {

	meterProvider := metric.NewMeterProvider()
	return meterProvider, nil
}

func newLoggerProvider() (*log.LoggerProvider, error) {
	exporter, err := stdoutlog.New(stdoutlog.WithPrettyPrint())

	if err != nil {
		return nil, err
	}

	loggerProvider := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(exporter)))
	return loggerProvider, nil
}
