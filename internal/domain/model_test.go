package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTransitions(t *testing.T) {
	cases := []struct {
		from, to, summary string
		want              error
	}{
		{"open", "acknowledged", "", nil}, {"open", "resolved", "corrigé", nil}, {"acknowledged", "resolved", "corrigé", nil},
		{"acknowledged", "open", "", ErrConflict}, {"resolved", "open", "", ErrConflict}, {"open", "resolved", "", ErrValidation},
		{"open", "other", "", ErrValidation}, {"open", "open", "inattendu", ErrValidation}, {"open", "acknowledged", "inattendu", ErrValidation},
	}
	now := time.Now().UTC()
	for _, tc := range cases {
		t.Run(tc.from+"_"+tc.to+"_"+tc.summary, func(t *testing.T) {
			i := Incident{Status: tc.from}
			err := Transition(&i, tc.to, tc.summary, now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("reçu %v, attendu %v", err, tc.want)
			}
			if err == nil {
				if i.Status != tc.to {
					t.Fatal("état non modifié")
				}
				if tc.to == "resolved" && (i.ResolvedAt == nil || !i.ResolvedAt.Equal(now) || i.ResolutionSummary != tc.summary) {
					t.Fatal("métadonnées de résolution absentes")
				}
				if tc.to == "acknowledged" && i.AcknowledgedAt == nil {
					t.Fatal("confirmation absente")
				}
			}
		})
	}
}
func TestServiceValidation(t *testing.T) {
	base := Service{Key: "video-api", Name: "Vidéo", Environment: "production", Status: "operational"}
	if !ValidService(base) {
		t.Fatal("service valide rejeté")
	}
	for _, key := range []string{"ab", "UPPER", "clé invalide", strings.Repeat("a", 65), "'; DROP TABLE services;--"} {
		s := base
		s.Key = key
		if ValidService(s) {
			t.Errorf("valeur acceptée : %q", key)
		}
	}
	base.Name = "   "
	if ValidService(base) {
		t.Fatal("nom vide accepté")
	}
}
