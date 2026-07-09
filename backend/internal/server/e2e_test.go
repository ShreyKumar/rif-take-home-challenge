// Black-box end-to-end tests: they drive the fully assembled server through
// its public HTTP surface (server.New + httptest.Server) against a real
// on-disk SQLite database, covering the cross-cutting paths unit tests can't
// reach — the submit → dedup → /stats/ lifecycle, invalid-input handling that
// must not persist, and counter persistence across a process restart.
package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/config"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/server"
)

// harness spins the assembled server against a fresh temp DB and returns a
// running httptest.Server plus the DB path (for restart tests).
func harness(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "e2e.db")
	cfg := config.Config{Port: "0", DBPath: dbPath, FrontendDir: t.TempDir()}
	handler, closer, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
		closer.Close()
	})
	return ts, dbPath
}

func postMutant(t *testing.T, ts *httptest.Server, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.URL+"/mutant/", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /mutant/: %v", err)
	}
	return resp
}

func getStats(t *testing.T, ts *httptest.Server) contract.StatsResponse {
	t.Helper()
	resp, err := http.Get(ts.URL + "/stats/")
	if err != nil {
		t.Fatalf("GET /stats/: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /stats/: status = %d, want 200", resp.StatusCode)
	}
	var s contract.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	return s
}

const (
	mutantDNA = `{"dna":["ATGCGA","CAGTGC","TTATGT","AGAAGG","CCCCTA","TCACTG"]}`
	humanDNA  = `{"dna":["ATGC","GCAT","TACG","CGTA"]}`
)

// TestE2ESubmitDedupStats exercises the full lifecycle: a mutant, a human, then
// resubmitting both, and asserts /stats/ reflects the deduplicated store.
func TestE2ESubmitDedupStats(t *testing.T) {
	ts, _ := harness(t)

	for _, tc := range []struct {
		body string
		want int
	}{
		{mutantDNA, http.StatusOK},
		{humanDNA, http.StatusForbidden},
		{mutantDNA, http.StatusOK},       // duplicate mutant
		{humanDNA, http.StatusForbidden}, // duplicate human
	} {
		resp := postMutant(t, ts, tc.body)
		if resp.StatusCode != tc.want {
			t.Fatalf("POST %s: status = %d, want %d", tc.body, resp.StatusCode, tc.want)
		}
		resp.Body.Close()
	}

	if s := getStats(t, ts); s.CountMutantDNA != 1 || s.CountHumanDNA != 1 || s.Ratio != 1.0 {
		t.Fatalf("stats = %+v, want {1, 1, 1.0} after dedup", s)
	}
}

// TestE2EInvalidInputsNotPersisted verifies every invalid path returns 400 and
// none of them affect the stored counts.
func TestE2EInvalidInputsNotPersisted(t *testing.T) {
	ts, _ := harness(t)

	for _, body := range []string{
		`{"dna":`,                               // malformed JSON
		`{}`,                                    // missing dna
		`{"dna":[]}`,                            // empty
		`{"dna":["ATGC","CAG"]}`,                // non-square
		`{"dna":["ATGX","CAGT","TTAT","AGAA"]}`, // illegal char
	} {
		resp := postMutant(t, ts, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("POST %q: status = %d, want 400", body, resp.StatusCode)
		}
		resp.Body.Close()
	}

	if s := getStats(t, ts); s.CountMutantDNA != 0 || s.CountHumanDNA != 0 {
		t.Fatalf("stats = %+v, want {0, 0} — invalid input must not persist", s)
	}
}

// TestE2EMethodNotAllowed checks the method contract end-to-end, including the
// Allow header, on both endpoints.
func TestE2EMethodNotAllowed(t *testing.T) {
	ts, _ := harness(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mutant/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mutant/: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != http.MethodPost {
		t.Fatalf("GET /mutant/: status=%d allow=%q, want 405 / POST", resp.StatusCode, resp.Header.Get("Allow"))
	}
	resp.Body.Close()

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/stats/", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /stats/: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed || resp.Header.Get("Allow") != http.MethodGet {
		t.Fatalf("POST /stats/: status=%d allow=%q, want 405 / GET", resp.StatusCode, resp.Header.Get("Allow"))
	}
	resp.Body.Close()
}

// TestE2EPersistenceAcrossRestart is the path unit tests can't reach: submit
// through the HTTP API, tear the whole server down, rebuild it against the same
// DB file, and confirm /stats/ still reports the persisted, deduplicated counts.
func TestE2EPersistenceAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	frontend := t.TempDir()

	// First lifetime: one mutant + one human (+ duplicates that must not count).
	first, closer1, err := server.New(config.Config{Port: "0", DBPath: dbPath, FrontendDir: frontend})
	if err != nil {
		t.Fatalf("server.New (first): %v", err)
	}
	ts1 := httptest.NewServer(first)
	for _, body := range []string{mutantDNA, humanDNA, mutantDNA} {
		resp, err := http.Post(ts1.URL+"/mutant/", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	ts1.Close()
	closer1.Close()

	// Second lifetime: same DB file, brand-new server + store.
	second, closer2, err := server.New(config.Config{Port: "0", DBPath: dbPath, FrontendDir: frontend})
	if err != nil {
		t.Fatalf("server.New (restart): %v", err)
	}
	defer closer2.Close()
	ts2 := httptest.NewServer(second)
	defer ts2.Close()

	if s := getStats(t, ts2); s.CountMutantDNA != 1 || s.CountHumanDNA != 1 {
		t.Fatalf("stats after restart = %+v, want {1, 1} — counts must survive a restart", s)
	}
}
