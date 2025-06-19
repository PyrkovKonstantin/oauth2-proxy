package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func newExporter(ctx context.Context, cfg *options.OTLP) (*otlptrace.Exporter, error) {
	switch cfg.Protocol {
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithURLPath("/v1/traces"),
		}

		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)

	case "grpc":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}

		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		return otlptracegrpc.New(ctx, opts...)

	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", cfg.Protocol)
	}
}

func newResource(ctx context.Context, appName string) (*resource.Resource, error) {
	return resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(appName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("application", fmt.Sprintf("/%s", appName)),
		),
	)
}

func newTraceProvider(resource *resource.Resource, spanProcessor sdktrace.SpanProcessor) *sdktrace.TracerProvider {

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(resource),
		sdktrace.WithSpanProcessor(spanProcessor),
	)

	return tracerProvider
}

func newMeterProvider(resource *resource.Resource) (*sdkmetric.MeterProvider, error) {
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(1*time.Minute),
		)),
	)

	return meterProvider, nil
}

func InitProvider(ctx context.Context, cfg *options.OTLP, appName string) (*sdktrace.TracerProvider, error) {
	exporter, err := newExporter(ctx, cfg)

	if err != nil {
		logger.Errorf("[OTEL] Failed to create the OTLP exporter: %v", err)
		return nil, err
	}

	resource, err := newResource(ctx, appName)

	if err != nil {
		logger.Errorf("[OTEL] Failed to create the OTLP resource: %v", err)
		return nil, err
	}

	meterProvider, err := newMeterProvider(resource)

	if err != nil {
		logger.Errorf("[OTEL] Failed to create the OTLP meter: %v", err)
	}
	batchSpanProcessor := sdktrace.NewBatchSpanProcessor(exporter)
	tp := newTraceProvider(resource, batchSpanProcessor)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
