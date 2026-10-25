package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"streamops/internal/config"
	"streamops/internal/domain"
	"streamops/internal/observability"
	"streamops/internal/service"
)

type API struct {
	app service.App
	cfg config.Config
	log *slog.Logger
}

func New(app service.App, cfg config.Config, log *slog.Logger, m *observability.Metrics) http.Handler {
	a := &API{app: app, cfg: cfg, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if app.Store.Ping(ctx) != nil {
			a.fail(w, r, 503, "not_ready", "base de données indisponible")
			return
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", a.metricsAuth(promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})))
	mux.HandleFunc("POST /v1/services", a.createService)
	mux.HandleFunc("GET /v1/services", a.listServices)
	mux.HandleFunc("GET /v1/services/{id}", a.getService)
	mux.HandleFunc("PATCH /v1/services/{id}", a.patchService)
	mux.HandleFunc("POST /v1/services/{id}/incidents", a.createIncident)
	mux.HandleFunc("GET /v1/incidents", a.listIncidents)
	mux.HandleFunc("GET /v1/incidents/{id}", a.getIncident)
	mux.HandleFunc("PATCH /v1/incidents/{id}", a.patchIncident)
	mux.HandleFunc("GET /v1/incidents/{id}/history", a.history)
	mux.HandleFunc("POST /v1/services/{id}/health-checks", a.createHealth)
	mux.HandleFunc("GET /v1/services/{id}/health-checks", a.listHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Recherche une autre méthode prise en charge pour distinguer une erreur 405 d’un chemin inconnu.
		for _, method := range []string{"GET", "POST", "PATCH"} {
			probe := r.Clone(r.Context())
			probe.Method = method
			_, pattern := mux.Handler(probe)
			if pattern != "/" && pattern != "" {
				allow := []string{}
				for _, candidate := range []string{"GET", "HEAD", "POST", "PATCH"} {
					p := r.Clone(r.Context())
					p.Method = candidate
					_, matched := mux.Handler(p)
					if matched != "/" && matched != "" {
						allow = append(allow, candidate)
					}
				}
				w.Header().Set("Allow", strings.Join(allow, ", "))
				a.fail(w, r, 405, "method_not_allowed", "méthode non autorisée")
				return
			}
		}
		a.fail(w, r, 404, "not_found", "route introuvable")
	})
	return m.Middleware(log, a.security(mux))
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *API) fail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": observability.RequestID(r.Context())}})
}
func (a *API) result(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err == nil {
		respond(w, status, v)
		return
	}
	switch {
	case errors.Is(err, domain.ErrValidation):
		a.fail(w, r, 400, "validation_error", "données ou transition d’incident invalides")
	case errors.Is(err, domain.ErrNotFound):
		a.fail(w, r, 404, "not_found", "ressource introuvable")
	case errors.Is(err, domain.ErrConflict):
		a.fail(w, r, 409, "conflict", "clé de service déjà utilisée ou transition d’incident interdite")
	case errors.Is(err, context.DeadlineExceeded):
		a.fail(w, r, 504, "timeout", "délai de l’opération dépassé")
	default:
		a.log.Error("échec de l’opération", "request_id", observability.RequestID(r.Context()), "error_type", strconv.QuoteToASCII(errorType(err)))
		a.fail(w, r, 500, "internal_error", "erreur interne du serveur")
	}
}

// Ne journalise pas les détails des erreurs SQL, susceptibles de contenir des données utilisateur.
func errorType(err error) string {
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	return "persistence_error"
}
func (a *API) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		a.fail(w, r, 415, "unsupported_media_type", "Content-Type doit être application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			a.fail(w, r, 413, "body_too_large", "le corps dépasse 32768 octets")
		} else {
			a.fail(w, r, 400, "invalid_json", "impossible de lire le corps JSON")
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		a.fail(w, r, 400, "invalid_json", "un seul objet JSON est attendu")
		return false
	}
	for _, value := range object {
		if string(value) == "null" {
			a.fail(w, r, 400, "invalid_json", "les champs null ne sont pas acceptés")
			return false
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(v) != nil {
		a.fail(w, r, 400, "invalid_json", "champs ou types JSON invalides")
		return false
	}
	return true
}
func (a *API) id(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	parsed, err := uuid.Parse(id)
	if err != nil {
		a.fail(w, r, 400, "validation_error", "id doit être un UUID")
		return "", false
	}
	return parsed.String(), true
}
func equal(token, expected string) bool {
	return len(expected) >= 32 && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}
func bearer(r *http.Request) string {
	prefix := "Bearer "
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, prefix) {
		return ""
	}
	return strings.TrimPrefix(v, prefix)
}
func (a *API) metricsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !equal(bearer(r), a.cfg.MetricsToken) {
			a.fail(w, r, 401, "unauthorized", "un jeton de métriques valide est requis")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *API) security(next http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Limit(a.cfg.Rate), a.cfg.Burst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if origin != a.cfg.CORSOrigin {
					a.fail(w, r, 403, "forbidden", "origine non autorisée")
					return
				}
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID,Location")
				if r.Method == "OPTIONS" {
					w.WriteHeader(204)
					return
				}
			}
			token := bearer(r)
			admin := equal(token, a.cfg.AdminToken)
			reader := equal(token, a.cfg.ReadToken)
			if !admin && !reader {
				w.Header().Set("WWW-Authenticate", `Bearer realm="streamops"`)
				a.fail(w, r, 401, "unauthorized", "un jeton Bearer valide est requis")
				return
			}
			if !admin && r.Method != "GET" && r.Method != "HEAD" {
				a.fail(w, r, 403, "forbidden", "un jeton d’administration est requis")
				return
			}
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				a.fail(w, r, 429, "rate_limited", "limite de requêtes dépassée")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), a.cfg.DBTimeout)
		defer cancel()
		*r = *r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
func (a *API) filter(w http.ResponseWriter, r *http.Request, kind string) (f domain.Filter, ok bool) {
	f.Limit = 20
	q := r.URL.Query()
	allowed := map[string]bool{"limit": true, "offset": true}
	switch kind {
	case "services":
		allowed["environment"] = true
		allowed["status"] = true
	case "incidents":
		for _, k := range []string{"service_id", "status", "severity", "from", "to"} {
			allowed[k] = true
		}
	}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 || values[0] == "" {
			a.fail(w, r, 400, "validation_error", "paramètre de requête inconnu, répété ou vide")
			return f, false
		}
	}
	var err error
	if q.Has("limit") {
		f.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			f.Limit = 0
		}
	}
	if q.Has("offset") {
		f.Offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil {
			f.Offset = -1
		}
	}
	valid := f.Limit >= 1 && f.Limit <= 100 && f.Offset >= 0 && f.Offset <= 1000000
	f.Environment = q.Get("environment")
	f.Status = q.Get("status")
	f.ServiceID = q.Get("service_id")
	f.Severity = q.Get("severity")
	if f.Environment != "" && !domain.OneOf(f.Environment, "development", "staging", "production") {
		valid = false
	}
	if f.Severity != "" && !domain.OneOf(f.Severity, "low", "medium", "high", "critical") {
		valid = false
	}
	if f.Status != "" {
		if kind == "services" {
			valid = valid && domain.OneOf(f.Status, "operational", "degraded", "outage", "maintenance")
		} else {
			valid = valid && domain.OneOf(f.Status, "open", "acknowledged", "resolved")
		}
	}
	if f.ServiceID != "" {
		parsed, e := uuid.Parse(f.ServiceID)
		if e != nil {
			valid = false
		} else {
			f.ServiceID = parsed.String()
		}
	}
	for key, target := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if v := q.Get(key); v != "" {
			t, e := time.Parse(time.RFC3339, v)
			if e != nil {
				valid = false
			} else {
				*target = &t
			}
		}
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		valid = false
	}
	if !valid {
		a.fail(w, r, 400, "validation_error", "pagination ou filtre invalide")
		return f, false
	}
	return f, true
}
func (a *API) createService(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key         string `json:"service_key"`
		Name        string `json:"name"`
		Environment string `json:"environment"`
		Status      string `json:"status"`
	}
	if !a.decode(w, r, &in) {
		return
	}
	s, err := a.app.CreateService(r.Context(), domain.Service{Key: in.Key, Name: in.Name, Environment: in.Environment, Status: in.Status})
	if err == nil {
		w.Header().Set("Location", "/v1/services/"+s.ID)
	}
	a.result(w, r, 201, s, err)
}
func (a *API) listServices(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filter(w, r, "services")
	if !ok {
		return
	}
	v, e := a.app.Store.ListServices(r.Context(), f)
	a.result(w, r, 200, v, e)
}
func (a *API) getService(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	v, e := a.app.Store.GetService(r.Context(), id)
	a.result(w, r, 200, v, e)
}
func (a *API) patchService(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	var p domain.ServicePatch
	if !a.decode(w, r, &p) {
		return
	}
	v, e := a.app.Store.PatchService(r.Context(), id, p)
	a.result(w, r, 200, v, e)
}
func (a *API) createIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Severity    string `json:"severity"`
	}
	if !a.decode(w, r, &in) {
		return
	}
	v, e := a.app.CreateIncident(r.Context(), domain.Incident{ServiceID: id, Title: in.Title, Description: in.Description, Severity: in.Severity})
	if e == nil {
		w.Header().Set("Location", "/v1/incidents/"+v.ID)
	}
	a.result(w, r, 201, v, e)
}
func (a *API) listIncidents(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filter(w, r, "incidents")
	if !ok {
		return
	}
	v, e := a.app.Store.ListIncidents(r.Context(), f)
	a.result(w, r, 200, v, e)
}
func (a *API) getIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	v, e := a.app.Store.GetIncident(r.Context(), id)
	a.result(w, r, 200, v, e)
}
func (a *API) patchIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	var p domain.IncidentPatch
	if !a.decode(w, r, &p) {
		return
	}
	v, e := a.app.Store.PatchIncident(r.Context(), id, p)
	a.result(w, r, 200, v, e)
}
func (a *API) history(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	v, e := a.app.Store.History(r.Context(), id)
	a.result(w, r, 200, v, e)
}
func (a *API) createHealth(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	var in struct {
		Status    string `json:"status"`
		LatencyMS int64  `json:"latency_ms"`
		Details   string `json:"details"`
	}
	if !a.decode(w, r, &in) {
		return
	}
	v, e := a.app.CreateHealthCheck(r.Context(), domain.HealthCheck{ServiceID: id, Status: in.Status, LatencyMS: in.LatencyMS, Details: in.Details})
	a.result(w, r, 201, v, e)
}
func (a *API) listHealth(w http.ResponseWriter, r *http.Request) {
	id, ok := a.id(w, r)
	if !ok {
		return
	}
	f, ok := a.filter(w, r, "health")
	if !ok {
		return
	}
	v, e := a.app.Store.ListHealthChecks(r.Context(), id, f)
	a.result(w, r, 200, v, e)
}
