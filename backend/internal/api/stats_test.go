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

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/stats/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
		if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
			t.Fatalf("%s: Allow = %q, want %q", method, allow, http.MethodGet)
		}
	}
}

// TestStatsRawJSONShape asserts the wire format directly — the exact JSON keys
// from the API contract (§3) and the literal numeric rendering of ratio —
// rather than round-tripping through contract.StatsResponse, which would pass
// even if the json tags or ratio formatting were wrong.
func TestStatsRawJSONShape(t *testing.T) {
	h := NewStatsHandler(statsStore{mutant: 40, human: 100}) // PDF 40/100 = 0.4
	req := httptest.NewRequest(http.MethodGet, "/stats/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body is not valid JSON: %v (%q)", err, rec.Body.String())
	}
	for _, key := range []string{"count_mutant_dna", "count_human_dna", "ratio"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("response missing key %q; body = %q", key, rec.Body.String())
		}
	}
	if got := string(raw["count_mutant_dna"]); got != "40" {
		t.Fatalf("count_mutant_dna = %s, want 40", got)
	}
	if got := string(raw["count_human_dna"]); got != "100" {
		t.Fatalf("count_human_dna = %s, want 100", got)
	}
	if got := string(raw["ratio"]); got != "0.4" {
		t.Fatalf("ratio rendered as %s, want 0.4", got)
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
