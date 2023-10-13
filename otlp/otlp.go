package otlp

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitOtlpProvider(ctx context.Context, res *resource.Resource) {
	client := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("127.0.0.1:4318"), otlptracehttp.WithInsecure())
	traceExporter, err := otlptrace.New(ctx, client)
	if err != nil {
		panic(fmt.Sprintf("creating OTLP trace exporter: %w", err))
	}

	// 暂时没有仔细看Collector的代码 Jaeger不支持Metric
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint("127.0.0.1:4318"), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(fmt.Sprintf("creating OTLP trace exporter: %w", err))
	}

	// 用Prometheus做临时代替
	//metricExporter, err := prometheus.New()
	otel.SetTracerProvider(newTraceProvider(traceExporter, res))
	//otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricExporter)))
	otel.SetMeterProvider(newMeterProvider(metricExporter))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}

func newTraceProvider(exp sdktrace.SpanExporter, res *resource.Resource) *sdktrace.TracerProvider {

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		//sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)), //概率
		//tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(0.5))),
	)
}

func newMeterProvider(exp sdkmetric.Exporter) *sdkmetric.MeterProvider {
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	return meterProvider
}
