package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"streamops/internal/domain"
	"streamops/internal/service"
)

type Postgres struct{ Pool *pgxpool.Pool }
type scanner interface{ Scan(...any) error }

const serviceCols = `id,service_key,name,environment,status,created_at,updated_at`
const incidentCols = `id,service_id,title,description,severity,status,opened_at,acknowledged_at,resolved_at,resolution_summary`
const healthCols = `id,service_id,status,checked_at,latency_ms,details`

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrNotFound
		case "23514", "22001", "22P02":
			return domain.ErrValidation
		}
	}
	return err
}
func scanService(r scanner) (s domain.Service, err error) {
	err = r.Scan(&s.ID, &s.Key, &s.Name, &s.Environment, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	return s, mapError(err)
}
func scanIncident(r scanner) (i domain.Incident, err error) {
	err = r.Scan(&i.ID, &i.ServiceID, &i.Title, &i.Description, &i.Severity, &i.Status, &i.OpenedAt, &i.AcknowledgedAt, &i.ResolvedAt, &i.ResolutionSummary)
	return i, mapError(err)
}
func scanHealth(r scanner) (h domain.HealthCheck, err error) {
	err = r.Scan(&h.ID, &h.ServiceID, &h.Status, &h.CheckedAt, &h.LatencyMS, &h.Details)
	return h, mapError(err)
}
func (p *Postgres) Ping(ctx context.Context) error { return p.Pool.Ping(ctx) }
func (p *Postgres) CreateService(ctx context.Context, s domain.Service) (domain.Service, error) {
	return scanService(p.Pool.QueryRow(ctx, `INSERT INTO services(id,service_key,name,environment,status) VALUES($1,$2,$3,$4,$5) RETURNING `+serviceCols, uuid.NewString(), s.Key, s.Name, s.Environment, s.Status))
}
func (p *Postgres) GetService(ctx context.Context, id string) (domain.Service, error) {
	return scanService(p.Pool.QueryRow(ctx, `SELECT `+serviceCols+` FROM services WHERE id=$1`, id))
}
func (p *Postgres) GetIncident(ctx context.Context, id string) (domain.Incident, error) {
	return scanIncident(p.Pool.QueryRow(ctx, `SELECT `+incidentCols+` FROM incidents WHERE id=$1`, id))
}

// list utilise un instantané repeatable-read commun au total et aux éléments retournés.
func list[T any](ctx context.Context, p *Postgres, f domain.Filter, table, cols, where, order string, args []any, scan func(scanner) (T, error)) (page domain.Page[T], err error) {
	page = domain.Page[T]{Items: []T{}, Limit: f.Limit, Offset: f.Offset}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return page, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+where, args...).Scan(&page.Total)
	if err != nil {
		return page, err
	}
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, cols, table, where, order, len(args)+1, len(args)+2)
	args = append(args, f.Limit, f.Offset)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		item, e := scan(rows)
		if e != nil {
			return page, e
		}
		page.Items = append(page.Items, item)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	rows.Close()
	return page, tx.Commit(ctx)
}
func (p *Postgres) ListServices(ctx context.Context, f domain.Filter) (domain.Page[domain.Service], error) {
	return list(ctx, p, f, "services", serviceCols, `($1='' OR environment=$1) AND ($2='' OR status=$2)`, "created_at DESC,id", []any{f.Environment, f.Status}, scanService)
}
func (p *Postgres) ListIncidents(ctx context.Context, f domain.Filter) (domain.Page[domain.Incident], error) {
	return list(ctx, p, f, "incidents", incidentCols, `($1='' OR service_id::text=$1) AND ($2='' OR status=$2) AND ($3='' OR severity=$3) AND ($4::timestamptz IS NULL OR opened_at >= $4) AND ($5::timestamptz IS NULL OR opened_at <= $5)`, "opened_at DESC,id", []any{f.ServiceID, f.Status, f.Severity, f.From, f.To}, scanIncident)
}
func (p *Postgres) PatchService(ctx context.Context, id string, patch domain.ServicePatch) (domain.Service, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.Service{}, err
	}
	defer tx.Rollback(ctx)
	s, err := scanService(tx.QueryRow(ctx, `SELECT `+serviceCols+` FROM services WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return s, err
	}
	if err = service.ApplyServicePatch(&s, patch); err != nil {
		return s, err
	}
	s, err = scanService(tx.QueryRow(ctx, `UPDATE services SET name=$2,environment=$3,status=$4,updated_at=clock_timestamp() WHERE id=$1 RETURNING `+serviceCols, id, s.Name, s.Environment, s.Status))
	if err != nil {
		return s, err
	}
	return s, tx.Commit(ctx)
}
func (p *Postgres) CreateIncident(ctx context.Context, i domain.Incident) (domain.Incident, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return i, err
	}
	defer tx.Rollback(ctx)
	i, err = scanIncident(tx.QueryRow(ctx, `INSERT INTO incidents(id,service_id,title,description,severity) VALUES($1,$2,$3,$4,$5) RETURNING `+incidentCols, uuid.NewString(), i.ServiceID, i.Title, i.Description, i.Severity))
	if err != nil {
		return i, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO incident_events(incident_id,from_status,to_status) VALUES($1,'','open')`, i.ID); err != nil {
		return i, err
	}
	return i, tx.Commit(ctx)
}
func (p *Postgres) PatchIncident(ctx context.Context, id string, patch domain.IncidentPatch) (domain.Incident, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return domain.Incident{}, err
	}
	defer tx.Rollback(ctx)
	i, err := scanIncident(tx.QueryRow(ctx, `SELECT `+incidentCols+` FROM incidents WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return i, err
	}
	old := i.Status
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return i, err
	}
	if err = service.ApplyIncidentPatch(&i, patch, now); err != nil {
		return i, err
	}
	i, err = scanIncident(tx.QueryRow(ctx, `UPDATE incidents SET title=$2,description=$3,severity=$4,status=$5,acknowledged_at=$6,resolved_at=$7,resolution_summary=$8 WHERE id=$1 RETURNING `+incidentCols, id, i.Title, i.Description, i.Severity, i.Status, i.AcknowledgedAt, i.ResolvedAt, i.ResolutionSummary))
	if err != nil {
		return i, mapError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO incident_events(incident_id,from_status,to_status,summary) VALUES($1,$2,$3,$4)`, id, old, i.Status, i.ResolutionSummary); err != nil {
		return i, err
	}
	return i, tx.Commit(ctx)
}
func (p *Postgres) History(ctx context.Context, id string) ([]domain.Event, error) {
	if _, err := p.GetIncident(ctx, id); err != nil {
		return nil, err
	}
	rows, err := p.Pool.Query(ctx, `SELECT id,incident_id,from_status,to_status,summary,occurred_at FROM incident_events WHERE incident_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Event{}
	for rows.Next() {
		var e domain.Event
		if err = rows.Scan(&e.ID, &e.IncidentID, &e.From, &e.To, &e.Summary, &e.At); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (p *Postgres) CreateHealthCheck(ctx context.Context, h domain.HealthCheck) (domain.HealthCheck, error) {
	return scanHealth(p.Pool.QueryRow(ctx, `INSERT INTO health_checks(id,service_id,status,latency_ms,details) VALUES($1,$2,$3,$4,$5) RETURNING `+healthCols, uuid.NewString(), h.ServiceID, h.Status, h.LatencyMS, h.Details))
}
func (p *Postgres) ListHealthChecks(ctx context.Context, id string, f domain.Filter) (domain.Page[domain.HealthCheck], error) {
	if _, err := p.GetService(ctx, id); err != nil {
		return domain.Page[domain.HealthCheck]{}, err
	}
	return list(ctx, p, f, "health_checks", healthCols, `service_id=$1`, "checked_at DESC,id", []any{id}, scanHealth)
}
