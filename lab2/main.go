package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	otelEndpoint = getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	serviceName  = getEnv("OTEL_SERVICE_NAME", "go-app")
	selfURL      = getEnv("SELF_URL", "http://localhost:1414")
	port         = getEnv("PORT", "1414")
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

func setupLogger() {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(&traceHandler{Handler: base}))
}

var tracer = otel.Tracer("go-app")

func setupTracing(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, status",
	},
	[]string{"method", "path", "status"},
)

var httpRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "path"},
)

func MetricsInit() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		method := r.Method
		path := r.URL.Path
		status := strconv.Itoa(rec.status)

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

		slog.InfoContext(r.Context(), "request",
			"method", method,
			"path", path,
			"status", rec.status,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

func instrument(h http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(
		metricsMiddleware(h),
		"http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

func main() {
	setupLogger()

	shutdownTracing, err := setupTracing(context.Background())
	if err != nil {
		log.Fatalf("tracing setup failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()

	MetricsInit()

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	mux.Handle("/health", instrument(handlerHealth))
	mux.Handle("/fail", instrument(handleFail))
	mux.Handle("/slow", instrument(handlerSlow))
	mux.Handle("/load", instrument(handlerLoad))

	log.Printf("starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleFail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	span := trace.SpanFromContext(ctx)
	span.SetStatus(codes.Error, "simulated failure")
	span.SetAttributes(attribute.String("error.type", "simulated"))

	slog.ErrorContext(ctx, "simulated failure",
		"path", r.URL.Path,
		"method", r.Method,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
}

func handlerSlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "slow-op")
	defer span.End()

	select {
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"slowAnswer"}`))
}

func handlerLoad(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var wg sync.WaitGroup

	for i := 0; i <= 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(selfURL + "/health")
			if err != nil {
				slog.WarnContext(ctx, "load request failed", "err", err)
				return
			}
			defer resp.Body.Close()
		}()
	}
	wg.Wait()

	slog.InfoContext(ctx, "load burst completed", "count", 41)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"WOW"}`))
}
