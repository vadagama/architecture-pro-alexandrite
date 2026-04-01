package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "simplest-collector:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("service-b"),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func main() {
	ctx := context.Background()

	tp, err := initTracer(ctx)
	if err != nil {
		log.Fatalf("failed to init tracer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
	}()

	tracer := otel.Tracer("service-b")

	http.Handle("/calculate", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := tracer.Start(r.Context(), "calculate-cost")
		defer span.End()

		// Simulate calculation work
		time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

		baseCost := 1000 + rand.Intn(9000)
		discount := rand.Intn(20)
		finalCost := float64(baseCost) * (1 - float64(discount)/100.0)

		span.SetAttributes(
			attribute.Int("calc.base_cost", baseCost),
			attribute.Int("calc.discount_pct", discount),
			attribute.Float64("calc.final_cost", finalCost),
		)

		log.Printf("Calculated cost: base=%d, discount=%d%%, final=%.2f", baseCost, discount, finalCost)

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"base_cost":%d,"discount_pct":%d,"final_cost":%.2f}`, baseCost, discount, finalCost)
	}), "calculate-cost"))

	log.Println("service-b listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
