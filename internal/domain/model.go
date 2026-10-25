package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrValidation = errors.New("échec de la validation")
	ErrNotFound   = errors.New("ressource introuvable")
	ErrConflict   = errors.New("conflit de ressource")
	keyPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,63}$`)
)

type Service struct {
	ID          string    `json:"id"`
	Key         string    `json:"service_key"`
	Name        string    `json:"name"`
	Environment string    `json:"environment"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Incident struct {
	ID                string     `json:"id"`
	ServiceID         string     `json:"service_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Severity          string     `json:"severity"`
	Status            string     `json:"status"`
	OpenedAt          time.Time  `json:"opened_at"`
	AcknowledgedAt    *time.Time `json:"acknowledged_at"`
	ResolvedAt        *time.Time `json:"resolved_at"`
	ResolutionSummary string     `json:"resolution_summary"`
}
type Event struct {
	ID         int64     `json:"id"`
	IncidentID string    `json:"incident_id"`
	From       string    `json:"from_status"`
	To         string    `json:"to_status"`
	Summary    string    `json:"summary"`
	At         time.Time `json:"occurred_at"`
}
type HealthCheck struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMS int64     `json:"latency_ms"`
	Details   string    `json:"details"`
}
type ServicePatch struct {
	Name        *string `json:"name"`
	Environment *string `json:"environment"`
	Status      *string `json:"status"`
}
type IncidentPatch struct {
	Title             *string `json:"title"`
	Description       *string `json:"description"`
	Severity          *string `json:"severity"`
	Status            *string `json:"status"`
	ResolutionSummary *string `json:"resolution_summary"`
}
type Filter struct {
	Limit       int
	Offset      int
	Environment string
	Status      string
	ServiceID   string
	Severity    string
	From        *time.Time
	To          *time.Time
}
type Page[T any] struct {
	Items  []T `json:"items"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

func OneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
func Text(v string, min, max int) bool { return len(strings.TrimSpace(v)) >= min && len(v) <= max }
func ValidService(s Service) bool {
	return keyPattern.MatchString(s.Key) && Text(s.Name, 1, 200) && OneOf(s.Environment, "development", "staging", "production") && OneOf(s.Status, "operational", "degraded", "outage", "maintenance")
}
func ValidIncident(i Incident) bool {
	return Text(i.Title, 1, 200) && Text(i.Description, 0, 10000) && OneOf(i.Severity, "low", "medium", "high", "critical") && Text(i.ResolutionSummary, 0, 4000)
}

// Transition autorise une résolution directe, mais interdit la réouverture d’un incident résolu.
func Transition(i *Incident, next, summary string, now time.Time) error {
	if !OneOf(next, "open", "acknowledged", "resolved") {
		return ErrValidation
	}
	if next == i.Status {
		if summary != i.ResolutionSummary {
			return ErrValidation
		}
		return nil
	}
	if i.Status == "resolved" || (i.Status == "acknowledged" && next == "open") {
		return ErrConflict
	}
	if next == "resolved" && !Text(summary, 1, 4000) {
		return ErrValidation
	}
	if next != "resolved" && summary != "" {
		return ErrValidation
	}
	if next == "acknowledged" {
		i.AcknowledgedAt = &now
	}
	if next == "resolved" {
		i.ResolvedAt = &now
		i.ResolutionSummary = summary
	}
	i.Status = next
	return nil
}
