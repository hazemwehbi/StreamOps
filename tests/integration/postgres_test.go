package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"streamops/internal/config"
	"streamops/internal/domain"
	httpapi "streamops/internal/http"
	"streamops/internal/observability"
	"streamops/internal/repository"
	"streamops/internal/service"
	"streamops/migrations"
)

func database(t *testing.T) *repository.Postgres {
	return databaseWithSetup(t, nil)
}
func databaseWithSetup(t *testing.T, setup func(context.Context, *pgxpool.Pool)) *repository.Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("définissez TEST_DATABASE_URL pour les tests PostgreSQL réels")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, e := admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
		if e != nil {
			t.Error(e)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if setup != nil {
		setup(ctx, pool)
	}
	for n := 0; n < 2; n++ {
		if e := migrations.Up(ctx, pool); e != nil {
			t.Fatal("migration/idempotence", e)
		}
	}
	if err = migrations.Verify(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return &repository.Postgres{Pool: pool}
}
func fixture(t *testing.T, p *repository.Postgres, key string) domain.Service {
	t.Helper()
	v, e := (service.App{Store: p}).CreateService(context.Background(), domain.Service{Key: key, Name: "Vidéo", Environment: "production"})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func ptr[T any](v T) *T { return &v }
func TestPostgresConstraintsFiltersAndAtomicity(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	a := service.App{Store: p}
	s := fixture(t, p, "video-api")
	if _, e := a.CreateService(ctx, s); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("clé en double", e)
	}
	for n := 0; n < 5; n++ {
		fixture(t, p, fmt.Sprintf("service-%d", n))
	}
	page, e := p.ListServices(ctx, domain.Filter{Limit: 2, Offset: 1, Environment: "production"})
	if e != nil || page.Total != 6 || len(page.Items) != 2 {
		t.Fatal(page, e)
	}
	empty, e := p.ListServices(ctx, domain.Filter{Limit: 2, Offset: 100})
	if e != nil || empty.Total != 6 || len(empty.Items) != 0 {
		t.Fatal(empty, e)
	}
	bad := domain.ServicePatch{Status: ptr("invalid")}
	if _, e := p.PatchService(ctx, s.ID, bad); !errors.Is(e, domain.ErrValidation) {
		t.Fatal(e)
	}
	unchanged, e := p.GetService(ctx, s.ID)
	if e != nil || unchanged.Status != "operational" {
		t.Fatal("échec de l’annulation de transaction", unchanged, e)
	}
	incident, e := a.CreateIncident(ctx, domain.Incident{ServiceID: s.ID, Title: "Mise en mémoire tampon", Severity: "critical"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.PatchIncident(ctx, incident.ID, domain.IncidentPatch{Status: ptr("resolved")}); !errors.Is(e, domain.ErrValidation) {
		t.Fatal(e)
	}
	events, e := p.History(ctx, incident.ID)
	if e != nil || len(events) != 1 {
		t.Fatal("une transition invalide a modifié l’historique", events, e)
	}
	// Les confirmations concurrentes sont sérialisées sans écraser l’horodatage.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := p.PatchIncident(ctx, incident.ID, domain.IncidentPatch{Status: ptr("acknowledged")})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	incident, e = p.PatchIncident(ctx, incident.ID, domain.IncidentPatch{Status: ptr("resolved"), ResolutionSummary: ptr("Encodeur redémarré")})
	if e != nil || incident.AcknowledgedAt == nil || incident.ResolvedAt == nil {
		t.Fatal(incident, e)
	}
	if _, e = p.PatchIncident(ctx, incident.ID, domain.IncidentPatch{Status: ptr("open")}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal(e)
	}
	events, e = p.History(ctx, incident.ID)
	if e != nil || len(events) != 4 || events[3].To != "resolved" {
		t.Fatal(events, e)
	}
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	incidents, e := p.ListIncidents(ctx, domain.Filter{Limit: 10, ServiceID: s.ID, Status: "resolved", Severity: "critical", From: &from, To: &to})
	if e != nil || incidents.Total != 1 {
		t.Fatal(incidents, e)
	}
	incidents, e = p.ListIncidents(ctx, domain.Filter{Limit: 10, ServiceID: s.ID, Status: "open"})
	if e != nil || incidents.Total != 0 {
		t.Fatal(incidents, e)
	}
	if _, e = a.CreateIncident(ctx, domain.Incident{ServiceID: uuid.NewString(), Title: "Absent", Severity: "low"}); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("clé étrangère", e)
	}
	if _, e = a.CreateHealthCheck(ctx, domain.HealthCheck{ServiceID: s.ID, Status: "healthy", LatencyMS: 12}); e != nil {
		t.Fatal(e)
	}
	checks, e := p.ListHealthChecks(ctx, s.ID, domain.Filter{Limit: 10})
	if e != nil || checks.Total != 1 || checks.Items[0].LatencyMS != 12 {
		t.Fatal(checks, e)
	}
	if _, e = p.Pool.Exec(ctx, `INSERT INTO health_checks(id,service_id,status,latency_ms) VALUES($1,$2,'healthy',-1)`, uuid.NewString(), s.ID); e == nil {
		t.Fatal("contrainte CHECK absente dans la base")
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, e = p.Pool.Exec(deadline, `SELECT pg_sleep(1)`); e == nil {
		t.Fatal("annulation ignorée par la base")
	}
}
func TestConcurrentDuplicateService(t *testing.T) {
	p := database(t)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := (service.App{Store: p}).CreateService(context.Background(), domain.Service{Key: "same-key", Name: "Identique", Environment: "staging"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, domain.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 7 {
		t.Fatal(success, conflict)
	}
}
func TestAPIWithPostgres(t *testing.T) {
	p := database(t)
	token := strings.Repeat("a", 32)
	h := httpapi.New(service.App{Store: p}, config.Config{AdminToken: token, ReadToken: strings.Repeat("b", 32), MetricsToken: strings.Repeat("c", 32), Rate: 100, Burst: 200, DBTimeout: 5 * time.Second}, slog.New(slog.NewJSONHandler(io.Discard, nil)), observability.NewMetrics())
	server := httptest.NewServer(h)
	defer server.Close()
	call := func(method, path, body string, status int, target any) {
		t.Helper()
		req, e := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		if resp.StatusCode != status {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		if target != nil {
			if e = json.NewDecoder(resp.Body).Decode(target); e != nil {
				t.Fatal(e)
			}
		}
	}
	var s domain.Service
	call("POST", "/v1/services", `{"service_key":"stream-api","name":"API de diffusion","environment":"staging"}`, 201, &s)
	call("POST", "/v1/services", `{"service_key":"stream-api","name":"Doublon","environment":"staging"}`, 409, nil)
	call("PATCH", "/v1/services/"+s.ID, `{"status":"degraded"}`, 200, &s)
	if s.Status != "degraded" {
		t.Fatal(s)
	}
	var i domain.Incident
	call("POST", "/v1/services/"+s.ID+"/incidents", `{"title":"Lecture indisponible","severity":"high"}`, 201, &i)
	call("PATCH", "/v1/incidents/"+i.ID, `{"status":"acknowledged"}`, 200, &i)
	call("PATCH", "/v1/incidents/"+i.ID, `{"status":"resolved","resolution_summary":"Capacité rétablie"}`, 200, &i)
	call("GET", "/v1/incidents?status=resolved&service_id="+s.ID, "", 200, nil)
	var history []domain.Event
	call("GET", "/v1/incidents/"+i.ID+"/history", "", 200, &history)
	if len(history) != 3 {
		t.Fatal(history)
	}
	call("POST", "/v1/services/"+s.ID+"/health-checks", `{"status":"healthy","latency_ms":18,"details":"HTTP 200"}`, 201, nil)
	call("GET", "/v1/services/"+s.ID+"/health-checks?limit=1", "", 200, nil)
	call("GET", "/v1/services/"+uuid.NewString(), "", 404, nil)
	call("GET", "/v1/incidents?from=bad", "", 400, nil)
}

func TestMigrationChecksumGuard(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if _, err := p.Pool.Exec(ctx, `UPDATE schema_migrations SET checksum='tampered' WHERE version='001_initial.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Verify(ctx, p.Pool); err == nil {
		t.Fatal("migration altérée acceptée au démarrage")
	}
	if err := migrations.Up(ctx, p.Pool); err == nil {
		t.Fatal("migration altérée acceptée par le moteur de migration")
	}
}

func TestRuntimeRolePrivileges(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("définissez TEST_DATABASE_URL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var exists bool
	if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='streamops_app')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Skip("le rôle streamops_app existant ne sera pas modifié")
	}
	if _, err = admin.Exec(ctx, `CREATE ROLE streamops_app NOLOGIN`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool, e := pgxpool.New(context.Background(), url)
		if e != nil {
			t.Error(e)
			return
		}
		defer pool.Close()
		if _, e = pool.Exec(context.Background(), `DROP ROLE streamops_app`); e != nil {
			t.Error(e)
		}
	})
	p := databaseWithSetup(t, func(ctx context.Context, pool *pgxpool.Pool) {
		var schema string
		if e := pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); e != nil {
			t.Fatal(e)
		}
		for _, sql := range []string{`GRANT USAGE ON SCHEMA ` + schema + ` TO streamops_app`, `ALTER DEFAULT PRIVILEGES IN SCHEMA ` + schema + ` GRANT SELECT,INSERT,UPDATE ON TABLES TO streamops_app`, `ALTER DEFAULT PRIVILEGES IN SCHEMA ` + schema + ` GRANT USAGE,SELECT ON SEQUENCES TO streamops_app`} {
			if _, e := pool.Exec(ctx, sql); e != nil {
				t.Fatal(e)
			}
		}
	})
	s := fixture(t, p, "role-service")
	for _, sql := range []string{`UPDATE schema_migrations SET checksum='bad'`, `INSERT INTO schema_migrations(version,checksum) VALUES('bad','bad')`, `UPDATE incident_events SET summary='bad'`, `CREATE TABLE forbidden(id int)`, `DELETE FROM services`} {
		tx, e := p.Pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, `SET LOCAL ROLE streamops_app`); e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, sql)
		_ = tx.Rollback(ctx)
		if e == nil {
			t.Errorf("opération autorisée au rôle d’exécution : %s", sql)
		}
	}
	tx, e := p.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SET LOCAL ROLE streamops_app`); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `UPDATE services SET status='maintenance' WHERE id=$1`, s.ID); e != nil {
		t.Fatal("le rôle d’exécution ne peut pas modifier les services", e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
}
