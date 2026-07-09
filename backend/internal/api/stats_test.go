package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
)

// statsStore is a fake Store with fixed counters; its name is scoped to this
// file to avoid colliding with the /mutant/ handler's fake.
type statsStore struct {
	mutant, human uint64
	err           error
}

func (s statsStore) Save(contract.Record) (bool, error) { return false, nil }

func (s statsStore) Stats() (uint64, uint64, error) {
	if s.err != nil {
		return 0, 0, s.err
	}
	return s.mutant, s.human, nil
}

func TestStatsEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		mutant    uint64
		human     uint64
		wantRatio float64
	}{
		{"empty", 0, 0, 0.0},
		{"pdf example 40 over 100", 40, 100, 0.4},
		{"zero humans is 0.0 not inf", 5, 0, 0.0}, // FR-4.4
		{"equal counts", 7, 7, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewStatsHandler(statsStore{mutant: tc.mutant, human: tc.human})
			req := httptest.NewRequest(http.MethodGet, "/stats/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var got contract.StatsResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got.CountMutantDNA != tc.mutant || got.CountHumanDNA != tc.human {
				t.Fatalf("counts = (%d, %d), want (%d, %d)", got.CountMutantDNA, got.CountHumanDNA, tc.mutant, tc.human)
			}
			if math.Abs(got.Ratio-tc.wantRatio) > 1e-9 {
				t.Fatalf("ratio = %v, want %v", got.Ratio, tc.wantRatio)
			}
		})
	}
}

func TestStatsRejectsNonGet(t *testing.T) {
	h := NewStatsHandler(statsStore{})
	req := httptest.NewRequest(http.MethodPost, "/stats/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestStatsStoreErrorIs500(t *testing.T) {
	h := NewStatsHandler(statsStore{err: errors.New("db unavailable")})
	req := httptest.NewRequest(http.MethodGet, "/stats/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
