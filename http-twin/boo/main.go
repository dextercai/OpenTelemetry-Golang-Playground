package main

import (
	"context"
	"github.com/gofiber/fiber/v2/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"time"
)

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	r := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("test"),
	)
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
		//sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)), //概率
		//tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(0.5))),
	)
}

func newMeterProvider(exp sdkmetric.Exporter) *sdkmetric.MeterProvider {
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	return meterProvider
}

func main() {
	client := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("127.0.0.1:4318"), otlptracehttp.WithInsecure())
	traceExporter, err := otlptrace.New(context.Background(), client)
	if err != nil {
		log.Fatalf("creating OTLP trace exporter: %w", err)
	}

	// 暂时没有仔细看Collector的代码 Jaeger不支持Metric
	metricExporter, err := otlpmetrichttp.New(context.Background(), otlpmetrichttp.WithEndpoint("127.0.0.1:4318"), otlpmetrichttp.WithInsecure())
	if err != nil {
		log.Fatalf("creating OTLP trace exporter: %w", err)
	}

	// 用Prometheus做临时代替
	//metricExporter, err := prometheus.New()
	otel.SetTracerProvider(newTraceProvider(traceExporter))
	//otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricExporter)))
	otel.SetMeterProvider(newMeterProvider(metricExporter))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	http.HandleFunc("/", indexHandler)
	http.ListenAndServe(":3000", nil)

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
	_, span := otel.Tracer("doHandleTracer").Start(ctx, "doHandle", trace.WithAttributes(attribute.String("url", r.URL.String())))

	bag := baggage.FromContext(ctx)
	defer span.End()
	span.AddEvent("doHandle 处理开始")
	span.AddEvent("baggage:" + bag.String())
	t := time.Now()
	span.SetAttributes(attribute.String("process.time", t.Sub(time.Now()).String()))
	w.Write([]byte("OK" + time.Now().String()))
}
