package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestOTLPExport(t *testing.T) {
	prior := otel.GetTracerProvider()
	priorPropagation := otel.GetTextMapPropagator()
	defer otel.SetTracerProvider(prior)
	defer otel.SetTextMapPropagator(priorPropagation)
	var exported atomic.Int32
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" || r.Method != "POST" {
			t.Error("route OTLP incorrecte", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		if len(data) == 0 {
			t.Error("export de traces vide")
		}
		exported.Add(1)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	}))
	defer collector.Close()
	shutdown, err := Tracing(context.Background(), collector.URL+"/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1}, TraceFlags: trace.FlagsSampled, Remote: true})
	_, span := otel.Tracer("test").Start(trace.ContextWithRemoteSpanContext(context.Background(), parent), "requête échantillonnée")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if exported.Load() == 0 {
		t.Fatal("aucun lot OTLP exporté")
	}
}
func TestPanicRecovery(t *testing.T) {
	h := NewMetrics().Middleware(slog.New(slog.NewJSONHandler(io.Discard, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("données confidentielles") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "données confidentielles") || w.Header().Get("X-Request-ID") == "" {
		t.Fatal(w)
	}
}
