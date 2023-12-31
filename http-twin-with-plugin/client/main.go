package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"io"
	"net/http"
	"time"
)

func main() {
	// applicationRes 通常一个服务实例共享同一个applicationRes
	// 用于记录服务名，服务节点名等信息
	applicationRes := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("httpClient-plugin"),
		semconv.K8SNodeName("single-node"),
	)

	ctx := context.Background()

	otlpClient := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("10.10.12.221:14318"), otlptracehttp.WithInsecure())

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

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint("10.10.12.221:14318"), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(fmt.Sprintf("creating OTLP metric exporter: %w", err))
	}
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter))))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// 初始化结束

	newCtx, span := otel.Tracer("requestTracer").Start(ctx, "httpReqStart")
	// defer span.End() 一般通过这种方式关闭Span，Demo由于只有单函数，并等待发送，该方法手动调用

	// 创建一个Baggage用于上下文传递
	bag, _ := baggage.New()
	member, _ := baggage.NewMember("user-id", "caiwenzhe")
	setMember, err := bag.SetMember(member)
	if err != nil {
		panic(err)
	}

	// 向Ctx中注入Baggage信息
	newCtx = baggage.ContextWithBaggage(newCtx, setMember)

	span.AddEvent("SendRequest")
	req, err := http.NewRequestWithContext(newCtx, "GET", "http://localhost:3000/api/do/123", nil)

	//carrier := propagation.HeaderCarrier(req.Header)
	//otel.GetTextMapPropagator().Inject(newCtx, carrier) // 注入到HttpHeader中进行传递
	//if err != nil {
	//	span.RecordError(err)
	//	return
	//}

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents))}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	fmt.Printf("%s", body)

	span.End()

	time.Sleep(10 * time.Second) // Trace一般后台发送，等待发送完。
}
