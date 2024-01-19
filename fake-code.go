ctx := context.Background()

// 初始化application Resource
// 描述服务基本信息，部署节点，等等。
applicationRes := resource.NewWithAttributes(
	semconv.SchemaURL,
	semconv.ServiceName("service-name"),
	semconv.K8SNodeName("suzhou-100"),
)

// 要发送trace到收集组件，所以需要初始化OtlpClient
// 有两种发送协议，HTTP/gRPC
// 这里初始化的是HTTP
otlpClient := otlptracehttp.NewClient() 

// 初始化traceExporter
traceExporter, err := otlptrace.New(ctx, otlpClient)

otel.SetTracerProvider(sdktrace.NewTracerProvider(
	sdktrace.WithBatcher(traceExporter), // 额外使用Batch发送方式，按需配置
	sdktrace.WithResource(applicationRes),
	//sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)), // 也可以配置采样概率等功能
	//tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(0.5))),
))

// 通过http otlpClient发送metric
metricExporter, err := otlpmetrichttp.New(ctx)

otel.SetMeterProvider(sdkmetric.NewMeterProvider(
	sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)) 
	// 我们套用了一个间隔（周期）发送
	)
)

// 传播器配置
otel.SetTextMapPropagator(
	propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, 
		propagation.Baggage{}
	)
)


// === 初始化完成

// 以Header为数据来源，初始化carrier
carrier := propagation.HeaderCarrier(r.Header) 
ctx = otel.GetTextMapPropagator().Extract(
	ctx, carrier
)
// 从Header中取出传播的信息，并注入到ctx中。
// 请注意 这里的ctx是Golang层面的，此时ctx中有了Context和Baggage

// 开始一个Span，分别制定TraceName、SpanName，便于后续筛选检索
// SpanName是否需要全局唯一？不限制
newCtx, span := otel.Tracer("trace-name").Start(ctx, "pack-debug-video") 

bag := baggage.FromContext(ctx) // 从ctx中获取Baggage信息

span.AddEvent("baggage:" + bag.String()) // 
span.SetAttributes(attribute.String("process.time", time.Now().String()) // 设置Span属性，可以便于后续检索

// 向baggage中增加一对KV
member, _ := baggage.NewMember("user-id", "caiwenzhe")
setMember, err := bag.SetMember(member)

// 由于Ctx是Golang最常用的上下文控制，所以我们需要继续更新Ctx
newCtx = baggage.ContextWithBaggage(newCtx, setMember)

req, err := http.NewRequestWithContext(newCtx, "GET", "http://localhost:3000/api/do/123", nil)
if err != nil {
	span.RecordError(err)
	return
}

carrier := propagation.HeaderCarrier(req.Header) // carrier是对req.Header的引用
otel.GetTextMapPropagator().Inject(newCtx, carrier) // 注入到HttpHeader中进行传递

client := http.Client{}
resp, err := client.Do(req) // 向下游进一步发送请求

span.End() // Span结束
