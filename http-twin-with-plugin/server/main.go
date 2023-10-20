package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

func main() {
	// applicationRes 通常一个服务实例共享同一个applicationRes
	// 用于记录服务名，服务节点名等信息
	applicationRes := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("httpServer-plugin"),
		semconv.K8SNodeName("single-node"),
	)

	ctx := context.Background()

	otlpClient := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("127.0.0.1:4318"), otlptracehttp.WithInsecure())

	traceExporter, err := otlptrace.New(ctx, otlpClient)
	if err != nil {
		panic(fmt.Sprintf("creating OTLP trace exporter: %w", err))
	}
	otel.SetTracerProvider(sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(applicationRes),
		//sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)),
		//tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(0.5))),
	))

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint("127.0.0.1:4318"), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(fmt.Sprintf("creating OTLP metric exporter: %w", err))
	}
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter))))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	http.Handle("/", otelhttp.NewHandler(http.HandlerFunc(indexHandler), "indexHandler", otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents)))
	http.ListenAndServe(":3000", nil)

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	bag := baggage.FromContext(ctx)

	// ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header)) //从Header中取出传播的信息

	ctx, span = otel.Tracer("doHandleTracer").Start(ctx, "doHandle", trace.WithAttributes(attribute.String("url", r.URL.String())))

	//bag := baggage.FromContext(ctx)
	defer span.End()
	span.AddEvent("doHandle 处理开始", trace.WithStackTrace(true))
	span.AddEvent("baggage got:"+bag.String(), trace.WithAttributes(attribute.String("user-id", bag.Member("user-id").String())))
	t := time.Now()
	span.SetAttributes(attribute.String("process.time", t.Sub(time.Now()).String()))

	counter, _ := otel.GetMeterProvider().Meter("httpServer").Int64Counter("indexHandlerCounter")
	counter.Add(ctx, 1)

	w.Write([]byte(time.Now().String()))
}
