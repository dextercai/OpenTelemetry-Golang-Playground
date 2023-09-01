package main

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"strings"
	"time"
)

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	r := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("ExampleService"),
	)
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)), //概率
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

	app := fiber.New()
	api := app.Group("/api")

	api.Use(middlewareHandle)
	RegisterApiGroup(api)

	api.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	log.Fatal(app.Listen(":3000"))
}

func middlewareHandle(c *fiber.Ctx) error {
	newCtx, span := otel.Tracer("middlewareHandleTracerName").Start(c.UserContext(), "apiMiddleware")

	counter, err := otel.Meter("middlewareHandleMeter").Int64Counter("middlewareHandleTimes")
	if err != nil {
		return err
	}
	counter.Add(c.UserContext(), 1)
	defer span.End()
	c.SetUserContext(newCtx)
	span.AddEvent("前置中间件")
	err = c.Next()
	span.AddEvent("后置中间件")
	return err
}

func RegisterApiGroup(router fiber.Router) {
	router.Get("/time/:input", timeHandle)
}

func timeHandle(c *fiber.Ctx) error {
	_, span := otel.Tracer("timeHandleTracerName").Start(c.UserContext(), "timeHandle", trace.WithAttributes(attribute.String("url", c.BaseURL())))
	defer span.End()
	span.AddEvent("timeHandle 入参校验")
	t := time.Now()
	if !strings.Contains(c.Params("input"), "hello") {
		span.RecordError(errors.New("输入未包含hello"))
	}
	span.SetAttributes(attribute.String("process.time", t.Sub(time.Now()).String()))

	return c.SendString("OK" + time.Now().String())
}
