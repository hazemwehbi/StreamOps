package observability

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type contextKey struct{}

func RequestID(ctx context.Context) string { v, _ := ctx.Value(contextKey{}).(string); return v }

type Metrics struct {
	Registry *prometheus.Registry
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry(), Requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "streamops_http_requests_total", Help: "Requêtes HTTP terminées."}, []string{"method", "route", "status"}), Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "streamops_http_duration_seconds", Help: "Durée des requêtes HTTP.", Buckets: prometheus.DefBuckets}, []string{"method", "route"})}
	m.Registry.MustRegister(m.Requests, m.Duration, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}

type writer struct {
	http.ResponseWriter
	status int
}

func (w *writer) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *writer) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (m *Metrics) Middleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := uuid.NewString()
		ctx := context.WithValue(r.Context(), contextKey{}, id)
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("streamops/http").Start(ctx, "Requête HTTP")
		defer span.End()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		wrapped := &writer{ResponseWriter: w}
		r = r.WithContext(ctx)
		defer func() {
			if recover() != nil {
				log.Error("panique pendant une requête", "request_id", id)
				if wrapped.status == 0 {
					wrapped.Header().Set("Content-Type", "application/json")
					wrapped.WriteHeader(500)
					_, _ = wrapped.Write([]byte(`{"error":{"code":"internal_error","message":"erreur interne du serveur","request_id":"` + id + `"}}`))
				}
			}
			status := wrapped.status
			if status == 0 {
				status = 200
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			method := r.Method
			if !oneMethod(method) {
				method = "OTHER"
			}
			span.SetName(route)
			span.SetAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, "erreur du serveur")
			}
			m.Requests.WithLabelValues(method, route, http.StatusText(status)).Inc()
			m.Duration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
			log.Info("requête HTTP", "request_id", id, "method", method, "route", route, "status", status, "duration_ms", time.Since(started).Milliseconds(), "trace_id", span.SpanContext().TraceID().String())
		}()
		next.ServeHTTP(wrapped, r)
	})
}
func oneMethod(m string) bool {
	switch m {
	case "GET", "HEAD", "POST", "PATCH", "PUT", "DELETE", "OPTIONS":
		return true
	}
	return false
}
func Tracing(ctx context.Context, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))), sdktrace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", "streamops"))))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return provider.Shutdown, nil
}
