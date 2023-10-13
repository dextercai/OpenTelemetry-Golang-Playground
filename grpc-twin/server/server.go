package main

import (
	"context"
	"fmt"
	"github.com/dextercai/OpenTelemetry-Golang-Playground/grpc-twin/opt"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

type serverImpl struct {
	*opt.UnimplementedTestServiceServer
}

func Init() {
	// applicationRes 通常一个服务实例共享同一个applicationRes
	// 用于记录服务名，服务节点名等信息
	applicationRes := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("grpcServer"),
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

func (s serverImpl) SayHello(ctx context.Context, request *opt.EchoRequest) (*opt.EchoReply, error) {
	ctx, span := otel.Tracer("grpcTracer").Start(ctx, "grpcSayHelloServerStart")
	defer span.End()
	span.AddEvent("Reply", trace.WithAttributes(
		attribute.String("username", "unknown")))
	rpy := &opt.EchoReply{Message: request.GetName()}
	return rpy, nil
}

func (s serverImpl) Add(ctx context.Context, request *opt.AddRequest) (*opt.AddReply, error) {
	ctx, span := otel.Tracer("grpcTracer").Start(ctx, "grpcAddServerStart")
	bag := baggage.FromContext(ctx)
	defer span.End()
	rpy := &opt.AddReply{}
	for i := range request.GetFoo() {
		rpy.Result += int64(i)
	}
	span.AddEvent("Done")
	span.AddEvent("user id from baggage:" + bag.Member("user-id").Value())

	return rpy, nil
}

func main() {
	Init()
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("监听端口失败: %s", err)
		return
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	opt.RegisterTestServiceServer(s, &serverImpl{})

	reflection.Register(s)

	err = s.Serve(lis)
	if err != nil {
		fmt.Printf("开启服务失败: %s", err)
		return
	}
}
