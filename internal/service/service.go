package service

import (
	"context"
	"streamops/internal/domain"
	"time"
)

// Store gère la persistance et les opérations atomiques de lecture, modification et écriture.
type Store interface {
	Ping(context.Context) error
	CreateService(context.Context, domain.Service) (domain.Service, error)
	GetService(context.Context, string) (domain.Service, error)
	ListServices(context.Context, domain.Filter) (domain.Page[domain.Service], error)
	PatchService(context.Context, string, domain.ServicePatch) (domain.Service, error)
	CreateIncident(context.Context, domain.Incident) (domain.Incident, error)
	GetIncident(context.Context, string) (domain.Incident, error)
	ListIncidents(context.Context, domain.Filter) (domain.Page[domain.Incident], error)
	PatchIncident(context.Context, string, domain.IncidentPatch) (domain.Incident, error)
	History(context.Context, string) ([]domain.Event, error)
	CreateHealthCheck(context.Context, domain.HealthCheck) (domain.HealthCheck, error)
	ListHealthChecks(context.Context, string, domain.Filter) (domain.Page[domain.HealthCheck], error)
}
type App struct{ Store Store }

func (a App) CreateService(ctx context.Context, s domain.Service) (domain.Service, error) {
	if s.Status == "" {
		s.Status = "operational"
	}
	if !domain.ValidService(s) {
		return s, domain.ErrValidation
	}
	return a.Store.CreateService(ctx, s)
}
func (a App) CreateIncident(ctx context.Context, i domain.Incident) (domain.Incident, error) {
	i.Status = "open"
	if !domain.ValidIncident(i) {
		return i, domain.ErrValidation
	}
	return a.Store.CreateIncident(ctx, i)
}
func (a App) CreateHealthCheck(ctx context.Context, h domain.HealthCheck) (domain.HealthCheck, error) {
	if !domain.OneOf(h.Status, "healthy", "unhealthy", "unknown") || h.LatencyMS < 0 || h.LatencyMS > 86400000 || !domain.Text(h.Details, 0, 2000) {
		return h, domain.ErrValidation
	}
	return a.Store.CreateHealthCheck(ctx, h)
}
func ApplyServicePatch(s *domain.Service, p domain.ServicePatch) error {
	if p.Name == nil && p.Environment == nil && p.Status == nil {
		return domain.ErrValidation
	}
	if p.Name != nil {
		s.Name = *p.Name
	}
	if p.Environment != nil {
		s.Environment = *p.Environment
	}
	if p.Status != nil {
		s.Status = *p.Status
	}
	if !domain.ValidService(*s) {
		return domain.ErrValidation
	}
	return nil
}

func ApplyIncidentPatch(i *domain.Incident, p domain.IncidentPatch, now time.Time) error {
	if p.Title == nil && p.Description == nil && p.Severity == nil && p.Status == nil && p.ResolutionSummary == nil {
		return domain.ErrValidation
	}
	if i.Status == "resolved" {
		return domain.ErrConflict
	}
	if p.Title != nil {
		i.Title = *p.Title
	}
	if p.Description != nil {
		i.Description = *p.Description
	}
	if p.Severity != nil {
		i.Severity = *p.Severity
	}
	next, summary := i.Status, i.ResolutionSummary
	if p.Status != nil {
		next = *p.Status
	}
	if p.ResolutionSummary != nil {
		summary = *p.ResolutionSummary
	}
	if err := domain.Transition(i, next, summary, now); err != nil {
		return err
	}
	if !domain.ValidIncident(*i) {
		return domain.ErrValidation
	}
	return nil
}
