package main

import (
	"context"
	"fmt"
	"github.com/dextercai/OpenTelemetry-Golang-Playground/grpc-twin/opt"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"

	"time"
)

func Init() {
	// applicationRes 通常一个服务实例共享同一个applicationRes
	// 用于记录服务名，服务节点名等信息
	applicationRes := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("grpcClient"),
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
	))

	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint("127.0.0.1:4318"), otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(fmt.Sprintf("creating OTLP metric exporter: %w", err))
	}
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter))))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}
func main() {
	Init()

	ctx := context.Background()
	dialOptions := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithInsecure(),
	}

	func(ctx context.Context) {
		ctx, span := otel.Tracer("grpcClientTracer").Start(ctx, "grpcSayHelloStart")
		defer span.End()

		conn, err := grpc.Dial(":8080", dialOptions...)
		if err != nil {
			fmt.Printf("连接服务端失败: %s", err)
			span.AddEvent("失败")

			return
		}

		defer conn.Close()
		time.Sleep(1 * time.Second)
		c := opt.NewTestServiceClient(conn)

		span.AddEvent("Req SayHello")

		r, err := c.SayHello(ctx, &opt.EchoRequest{Name: "Sato"})
		if err != nil {
			fmt.Printf("调用服务端代码失败: %s", err)
			span.SetStatus(codes.Error, "连接服务端失败")
			span.RecordError(err)

			return
		}
		span.AddEvent("Req Add")
		bag, _ := baggage.New()
		member, err := baggage.NewMember("user-id", "caiwenzhe")
		setMember, err := bag.SetMember(member)

		if err != nil {
			panic(err)
		}
		ctx = baggage.ContextWithBaggage(ctx, setMember)

		c.Add(ctx, &opt.AddRequest{Foo: []int32{
			1, 2, 3, 4, 5, 6, 7,
		}})

		fmt.Printf("调用成功: %s", r.Message)
	}(ctx)

	time.Sleep(10 * time.Second)

}
