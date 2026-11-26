package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultPort         = "8080"
	defaultServiceName  = "service-b"
	defaultOTLPEndpoint = "http://simplest-collector:4318"
	shutdownTimeout     = 5 * time.Second
)

type priceResponse struct {
	Service  string `json:"service"`
	Price    int    `json:"price"`
	Currency string `json:"currency"`
	TraceID  string `json:"trace_id"`
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func initTracer(ctx context.Context, serviceName string) (*sdktrace.TracerProvider, error) {
	opts := []otlptracehttp.Option{}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(defaultOTLPEndpoint))
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}

func main() {
	serviceName := envOrDefault("OTEL_SERVICE_NAME", defaultServiceName)
	port := envOrDefault("PORT", defaultPort)

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", serviceName)
	slog.SetDefault(log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	tp, err := initTracer(ctx, serviceName)
	if err != nil {
		log.Error("failed to init tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Error("tracer shutdown error", "error", err)
		}
	}()

	tracer := otel.Tracer(serviceName)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "price.calculate")
		defer span.End()

		time.Sleep(20 * time.Millisecond)

		price := 45000
		span.SetAttributes(
			attribute.Int("model.polygons", 12500),
			attribute.Int("price.value", price),
		)

		traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
		result := priceResponse{
			Service:  serviceName,
			Price:    price,
			Currency: "RUB",
			TraceID:  traceID,
		}

		log.Info("price calculated", "price", price, "trace_id", traceID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: otelhttp.NewHandler(mux, "GET /"),
	}

	go func() {
		log.Info("listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", "error", err)
	}
}
