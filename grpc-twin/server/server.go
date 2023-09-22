package main

import (
	"context"
	"fmt"
	"github.com/dextercai/OpenTelemetry-Golang-Playground/grpc-twin/opt"
	"github.com/dextercai/OpenTelemetry-Golang-Playground/otlp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

type serverImpl struct {
	*opt.UnimplementedTestServiceServer
}

func (s serverImpl) SayHello(ctx context.Context, request *opt.EchoRequest) (*opt.EchoReply, error) {

	ctx, span := otel.Tracer("grpcTracer").Start(ctx, "grpcSayHelloServerStart")
	defer span.End()
	span.AddEvent("Reply")
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
	span.AddEvent("user id" + bag.Member("user-id").Value())

	return rpy, nil
}

func main() {
	otlp.InitOtlpProvider(context.Background())
	// 监听本地端口
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("监听端口失败: %s", err)
		return
	}

	s := grpc.NewServer(grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
		otelgrpc.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
	)))
	opt.RegisterTestServiceServer(s, &serverImpl{})

	reflection.Register(s)

	err = s.Serve(lis)
	if err != nil {
		fmt.Printf("开启服务失败: %s", err)
		return
	}
}
