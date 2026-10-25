package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streamops/internal/config"
	"streamops/internal/domain"
	"streamops/internal/observability"
	"streamops/internal/service"
)

const admin = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const reader = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

type stub struct {
	service.Store
	pingErr   error
	createErr error
}

func (s stub) Ping(context.Context) error { return s.pingErr }
func (s stub) CreateService(_ context.Context, v domain.Service) (domain.Service, error) {
	v.ID = "b9e047a1-d44a-4edf-a98b-572dd6cb9ab6"
	return v, s.createErr
}
func (s stub) ListServices(_ context.Context, f domain.Filter) (domain.Page[domain.Service], error) {
	return domain.Page[domain.Service]{Items: []domain.Service{}, Limit: f.Limit, Offset: f.Offset}, nil
}
func handler(s stub) http.Handler {
	return New(service.App{Store: s}, config.Config{AdminToken: admin, ReadToken: reader, MetricsToken: strings.Repeat("c", 32), DBTimeout: time.Second, Rate: 1000, Burst: 1000}, slog.New(slog.NewJSONHandler(io.Discard, nil)), observability.NewMetrics())
}
func request(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHTTPContract(t *testing.T) {
	h := handler(stub{})
	cases := []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", "/healthz", "", "", 200}, {"GET", "/readyz", "", "", 200},
		{"GET", "/v1/services", "", "", 401}, {"POST", "/v1/services", `{}`, reader, 403},
		{"GET", "/v1/services?limit=0", "", admin, 400}, {"GET", "/v1/services?limit=2&limit=3", "", admin, 400},
		{"GET", "/v1/services?status=bogus", "", admin, 400}, {"GET", "/v1/services?unknown=1", "", admin, 400},
		{"GET", "/v1/services/not-a-uuid", "", admin, 400}, {"GET", "/v1/services?limit=5", "", reader, 200},
		{"POST", "/v1/services", `{"service_key":"video","name":"Vidéo","environment":"production"}`, admin, 201},
		{"POST", "/v1/services", `{"service_key":"video","name":"Vidéo","environment":"production","id":"client"}`, admin, 400},
		{"POST", "/v1/services", `null`, admin, 400}, {"POST", "/v1/services", `{"name":null}`, admin, 400},
		{"POST", "/v1/services", `{} {}`, admin, 400}, {"POST", "/v1/services", `{}`, admin, 400},
		{"GET", "/metrics", "", "", 401}, {"GET", "/missing", "", "", 404},
		{"DELETE", "/v1/services", "", admin, 405},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			w := request(h, tc.method, tc.path, tc.body, tc.token)
			if w.Code != tc.status {
				t.Fatalf("reçu %d, attendu %d : %s", w.Code, tc.status, w.Body)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("identifiant de requête absent")
			}
			var body map[string]any
			if json.Unmarshal(w.Body.Bytes(), &body) != nil {
				t.Fatal("réponse JSON invalide")
			}
			if tc.status >= 400 && body["error"] == nil {
				t.Fatal("enveloppe d’erreur absente")
			}
		})
	}
}
func TestFailuresDoNotLeakSecrets(t *testing.T) {
	h := handler(stub{pingErr: errors.New("DSN confidentiel"), createErr: errors.New("DSN confidentiel")})
	if w := request(h, "GET", "/readyz", "", ""); w.Code != 503 || strings.Contains(w.Body.String(), "DSN confidentiel") {
		t.Fatal(w)
	}
	w := request(h, "POST", "/v1/services", `{"service_key":"video","name":"Vidéo","environment":"production"}`, admin)
	if w.Code != 500 || strings.Contains(w.Body.String(), "DSN confidentiel") {
		t.Fatal(w)
	}
}

func TestRateLimitAndMediaType(t *testing.T) {
	cfg := config.Config{AdminToken: admin, ReadToken: reader, Rate: 0.001, Burst: 1, DBTimeout: time.Second}
	h := New(service.App{Store: stub{}}, cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)), observability.NewMetrics())
	if w := request(h, "GET", "/v1/services", "", reader); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := request(h, "GET", "/v1/services", "", reader); w.Code != 429 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("POST", "/v1/services", strings.NewReader("{}"))
	r.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	handler(stub{}).ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal(w.Code)
	}
}
func TestBodyLimitCORSAndMetrics(t *testing.T) {
	h := handler(stub{})
	if w := request(h, "POST", "/v1/services", `{"name":"`+strings.Repeat("x", 33000)+`"}`, admin); w.Code != 413 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", "/v1/services", nil)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	_ = request(h, "GET", "/v1/services?limit=5", "", reader)
	w = request(h, "GET", "/metrics", "", strings.Repeat("c", 32))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `route="GET /v1/services"`) {
		t.Fatal("métriques de route bornées absentes", w.Body.String())
	}
}
