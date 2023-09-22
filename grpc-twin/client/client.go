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
	"net/http"
	"time"
)

func main() {
	ctx := context.Background()
	otlp.InitOtlpProvider(ctx)

	http.HandleFunc("/", indexHandler)
	http.ListenAndServe(":8001", nil)

}

func indexHandler(writer http.ResponseWriter, request *http.Request) {
	// 连接服务器
	ctx := request.Context()
	dialOptions := []grpc.DialOption{
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(
			otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
			otelgrpc.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		)),
		grpc.WithInsecure(),
	}
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

	//propagators := propagation.TraceContext{}
	// propagators.Inject(ctx, )
	span.AddEvent("Req SayHello")

	r, err := c.SayHello(ctx, &opt.EchoRequest{Name: "Sato"})
	if err != nil {
		fmt.Printf("调用服务端代码失败: %s", err)
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
}
