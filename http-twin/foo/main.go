package main

import (
	"context"
	"fmt"
	"github.com/dextercai/OpenTelemetry-Golang-Playground/otlp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"io"
	"net/http"
)

func main() {
	otlp.InitOtlpProvider(context.TODO())
	http.HandleFunc("/", indexHandler)
	http.ListenAndServe(":3001", nil)

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	newCtx, span := otel.Tracer("requestTracer").Start(ctx, "api/do/123")
	defer span.End()
	bag, _ := baggage.New()
	member, err := baggage.NewMember("user-id", "caiwenzhe")
	setMember, err := bag.SetMember(member)
	if err != nil {
		panic(err)
	}
	newCtx = baggage.ContextWithBaggage(newCtx, setMember)

	//otel.GetTextMapPropagator().Inject(newCtx, propagation.HeaderCarrier(req.Header))

	req, err := http.NewRequestWithContext(newCtx, "GET", "http://localhost:3000/api/do/123", nil)
	carrier := propagation.HeaderCarrier(req.Header)
	otel.GetTextMapPropagator().Inject(newCtx, carrier)

	if err != nil {
		panic(err)
	}
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	fmt.Printf("%s", body)

}
