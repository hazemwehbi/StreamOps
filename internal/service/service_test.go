package service

import (
	"context"
	"errors"
	"streamops/internal/domain"
	"testing"
)

type neverStore struct{ Store }

func TestInvalidInputNeverReachesPersistence(t *testing.T) {
	a := App{Store: neverStore{}}
	if _, e := a.CreateService(context.Background(), domain.Service{}); !errors.Is(e, domain.ErrValidation) {
		t.Fatal(e)
	}
	if _, e := a.CreateIncident(context.Background(), domain.Incident{}); !errors.Is(e, domain.ErrValidation) {
		t.Fatal(e)
	}
	for _, h := range []domain.HealthCheck{{Status: "healthy", LatencyMS: -1}, {Status: "invalid"}, {Status: "healthy", LatencyMS: 86400001}} {
		if _, e := a.CreateHealthCheck(context.Background(), h); !errors.Is(e, domain.ErrValidation) {
			t.Fatal(e)
		}
	}
}
func TestServicePatch(t *testing.T) {
	s := domain.Service{Key: "abc", Name: "avant", Environment: "staging", Status: "operational"}
	name := "après"
	if e := ApplyServicePatch(&s, domain.ServicePatch{Name: &name}); e != nil || s.Name != "après" || s.Key != "abc" {
		t.Fatal(s, e)
	}
	if !errors.Is(ApplyServicePatch(&s, domain.ServicePatch{}), domain.ErrValidation) {
		t.Fatal("modification vide acceptée")
	}
}
