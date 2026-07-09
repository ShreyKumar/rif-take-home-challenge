package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/config"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/mutant"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/store"
)

// realHandler assembles the handler with the real detector and a fresh
// in-memory store, so the smoke tests exercise the actual wiring end to end.
func realHandler(t *testing.T, frontendDir string) http.Handler {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New(:memory:): %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return Assemble(mutant.IsMutant, st, frontendDir)
}

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(realHandler(t, t.TempDir()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q, want %q", body, "ok")
	}
}

// TestMutantStatsWiring drives the assembled handler through the full lifecycle:
// a real mutant → 200, a human → 403, a duplicate mutant that must not
// double-count, and /stats/ reflecting the deduplicated counters.
func TestMutantStatsWiring(t *testing.T) {
	srv := httptest.NewServer(realHandler(t, t.TempDir()))
	defer srv.Close()

	post := func(body string) *http.Response {
		t.Helper()
		resp, err := http.Post(srv.URL+"/mutant/", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /mutant/: %v", err)
		}
		return resp
	}

	mutantDNA := `{"dna":["ATGCGA","CAGTGC","TTATGT","AGAAGG","CCCCTA","TCACTG"]}`
	humanDNA := `{"dna":["ATGC","GCAT","TACG","CGTA"]}`

	// Mutant → 200, {"mutant":true}
	resp := post(mutantDNA)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mutant: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var mr contract.MutantResponse
	json.NewDecoder(resp.Body).Decode(&mr)
	resp.Body.Close()
	if !mr.Mutant {
		t.Fatal("mutant: body.mutant = false, want true")
	}

	// Human → 403
	resp = post(humanDNA)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("human: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	resp.Body.Close()

	// Duplicate mutant → still 200, but must not double-count in stats.
	resp = post(mutantDNA)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dup mutant: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()

	// Stats reflect the deduplicated store: 1 mutant, 1 human, ratio 1.0.
	resp, err := http.Get(srv.URL + "/stats/")
	if err != nil {
		t.Fatalf("GET /stats/: %v", err)
	}
	var sr contract.StatsResponse
	json.NewDecoder(resp.Body).Decode(&sr)
	resp.Body.Close()
	if sr.CountMutantDNA != 1 || sr.CountHumanDNA != 1 || sr.Ratio != 1.0 {
		t.Fatalf("stats = %+v, want {1, 1, 1.0}", sr)
	}
}

// TestStaticServing verifies the file server is mounted at / over frontendDir.
func TestStaticServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>mutant detector</h1>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	srv := httptest.NewServer(realHandler(t, dir))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "mutant detector") {
		t.Fatalf("GET / body = %q, want it to contain the index.html content", body)
	}
}

func TestUnknownPathNotFound(t *testing.T) {
	// An empty frontend dir means the file server 404s on any missing path.
	srv := httptest.NewServer(realHandler(t, t.TempDir()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET /does-not-exist: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestNewFromConfig covers the production wiring path: New opens a real SQLite
// file from config and returns a working handler plus a closeable store.
func TestNewFromConfig(t *testing.T) {
	cfg := config.Config{
		Port:        "0",
		DBPath:      filepath.Join(t.TempDir(), "mutant.db"),
		FrontendDir: t.TempDir(),
	}
	handler, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New(cfg): %v", err)
	}
	defer closer.Close()

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestNewStoreOpenError verifies New surfaces a store-open failure instead of
// panicking (e.g. a DB path inside a nonexistent directory).
func TestNewStoreOpenError(t *testing.T) {
	cfg := config.Config{
		DBPath:      filepath.Join(t.TempDir(), "no-such-dir", "mutant.db"),
		FrontendDir: t.TempDir(),
	}
	if _, _, err := New(cfg); err == nil {
		t.Fatal("New with unwritable DBPath returned nil error, want failure")
	}
}
