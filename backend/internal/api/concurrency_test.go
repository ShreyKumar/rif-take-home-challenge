// Concurrency stress test (external package api_test): it drives the real
// /mutant/ + /stats/ handlers — wired to the real SQLite store and the real
// detector — via httptest.Server under a bounded pool of goroutines that mix
// duplicate submissions (dedup contention), many distinct submissions, and
// concurrent /stats/ reads. It asserts INVARIANTS, not timings: exactly one
// row per distinct DNA, count_mutant + count_human == distinct DNAs, a ratio
// consistent with the split, and zero 5xx. Runs inside `go test -race`.
package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/api"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/mutant"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/store"
)

const bases = "ATCG"

// genDNA returns a distinct, valid 4×4 A/T/C/G matrix for each n: row 0 encodes
// n's low 8 bits, so matrices are unique for n < 256; the fixed filler rows keep
// every matrix valid. Mutant status is whatever the real detector decides.
func genDNA(n int) []string {
	row0 := make([]byte, 4)
	for c := 0; c < 4; c++ {
		row0[c] = bases[(n>>(2*c))&3]
	}
	return []string{string(row0), "CGAT", "TACG", "GATC"}
}

func TestConcurrentDedupInvariants(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()

	mux := http.NewServeMux()
	mux.Handle("/mutant/", api.NewMutantHandler(mutant.IsMutant, st))
	mux.Handle("/stats/", api.NewStatsHandler(st))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	const (
		distinct   = 120 // distinct DNAs
		duplicates = 6   // times each is submitted (concurrent dedup contention)
	)

	// Expected split computed locally over the distinct set.
	var wantMutant, wantHuman uint64
	bodies := make([]string, distinct)
	for n := 0; n < distinct; n++ {
		dna := genDNA(n)
		if mutant.IsMutant(dna) {
			wantMutant++
		} else {
			wantHuman++
		}
		b, _ := json.Marshal(contract.MutantRequest{DNA: dna})
		bodies[n] = string(b)
	}

	var status5xx, transportErr int64
	sem := make(chan struct{}, 48) // bound in-flight requests
	var wg sync.WaitGroup

	post := func(body string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()
		resp, err := http.Post(ts.URL+"/mutant/", "application/json", strings.NewReader(body))
		if err != nil {
			atomic.AddInt64(&transportErr, 1)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 500 {
			atomic.AddInt64(&status5xx, 1)
		}
	}
	readStats := func() {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()
		resp, err := http.Get(ts.URL + "/stats/")
		if err != nil {
			atomic.AddInt64(&transportErr, 1)
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 500 {
			atomic.AddInt64(&status5xx, 1)
		}
	}

	// Fire all duplicates of every distinct DNA concurrently, interleaved with
	// concurrent /stats/ reads.
	for d := 0; d < duplicates; d++ {
		for n := 0; n < distinct; n++ {
			wg.Add(1)
			go post(bodies[n])
		}
		for r := 0; r < 20; r++ {
			wg.Add(1)
			go readStats()
		}
	}
	wg.Wait()

	if status5xx != 0 {
		t.Fatalf("observed %d responses with status >= 500, want 0", status5xx)
	}
	if transportErr != 0 {
		t.Fatalf("%d requests failed at the transport level, want 0", transportErr)
	}

	// Final invariants from the maintained counters.
	resp, err := http.Get(ts.URL + "/stats/")
	if err != nil {
		t.Fatalf("final GET /stats/: %v", err)
	}
	defer resp.Body.Close()
	var s contract.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	if got := s.CountMutantDNA + s.CountHumanDNA; got != distinct {
		t.Fatalf("count_mutant + count_human = %d, want %d (exactly one row per distinct DNA)", got, distinct)
	}
	if s.CountMutantDNA != wantMutant || s.CountHumanDNA != wantHuman {
		t.Fatalf("stats = (%d mutant, %d human), want (%d, %d)", s.CountMutantDNA, s.CountHumanDNA, wantMutant, wantHuman)
	}
	var wantRatio float64
	if wantHuman > 0 {
		wantRatio = float64(wantMutant) / float64(wantHuman)
	}
	if s.Ratio != wantRatio {
		t.Fatalf("ratio = %v, want %v", s.Ratio, wantRatio)
	}
}
