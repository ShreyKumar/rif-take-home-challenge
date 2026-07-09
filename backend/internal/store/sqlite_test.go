package store

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
)

// newMemStore returns an isolated in-memory store for a single test.
func newMemStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:): %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustStats(t *testing.T, s *SQLiteStore) (mutant, human uint64) {
	t.Helper()
	m, h, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	return m, h
}

func TestSaveInsertsAndCounts(t *testing.T) {
	s := newMemStore(t)

	inserted, err := s.Save(contract.Record{DNA: "AAAA\nGGGG", IsMutant: true})
	if err != nil {
		t.Fatalf("Save mutant: %v", err)
	}
	if !inserted {
		t.Fatal("first mutant Save: inserted = false, want true")
	}
	if _, err := s.Save(contract.Record{DNA: "ATGC\nGCAT", IsMutant: false}); err != nil {
		t.Fatalf("Save human: %v", err)
	}

	if m, h := mustStats(t, s); m != 1 || h != 1 {
		t.Fatalf("stats = (%d mutant, %d human), want (1, 1)", m, h)
	}
}

func TestSaveDeduplicates(t *testing.T) {
	s := newMemStore(t)
	rec := contract.Record{DNA: "AAAA\nGGGG", IsMutant: true}

	inserted, err := s.Save(rec)
	if err != nil || !inserted {
		t.Fatalf("first Save: inserted=%v err=%v, want true/nil", inserted, err)
	}

	inserted, err = s.Save(rec)
	if err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if inserted {
		t.Fatal("resubmitting the same DNA reported inserted=true, want false")
	}

	// Counters must bump exactly once despite two submissions.
	if m, h := mustStats(t, s); m != 1 || h != 0 {
		t.Fatalf("stats = (%d, %d), want (1, 0) after dedup", m, h)
	}
}

func TestStatsEmpty(t *testing.T) {
	s := newMemStore(t)
	if m, h := mustStats(t, s); m != 0 || h != 0 {
		t.Fatalf("empty stats = (%d, %d), want (0, 0)", m, h)
	}
}

// TestConcurrentDuplicateSave verifies FR-3.3: a race of identical DNA still
// yields exactly one inserted row and a single counter bump.
func TestConcurrentDuplicateSave(t *testing.T) {
	s := newMemStore(t)
	rec := contract.Record{DNA: "CCCC\nGGGG\nTTTT\nAAAA", IsMutant: true}

	const goroutines = 64
	var wg sync.WaitGroup
	var mu sync.Mutex
	inserts := 0
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			inserted, err := s.Save(rec)
			if err != nil {
				t.Errorf("concurrent Save: %v", err)
				return
			}
			if inserted {
				mu.Lock()
				inserts++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if inserts != 1 {
		t.Fatalf("concurrent duplicate Saves inserted %d rows, want exactly 1", inserts)
	}
	if m, h := mustStats(t, s); m != 1 || h != 0 {
		t.Fatalf("stats after concurrent dedup = (%d, %d), want (1, 0)", m, h)
	}
}

// TestPersistenceAcrossReopen verifies counters are re-seeded from an existing
// database file (FR-3.4 / restart safety).
func TestPersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mutant.db")

	first, err := New(path)
	if err != nil {
		t.Fatalf("New(first): %v", err)
	}
	for _, rec := range []contract.Record{
		{DNA: "AAAA\nCCCC\nGGGG\nTTTT", IsMutant: true},
		{DNA: "ATGC\nGCAT\nTACG\nCGTA", IsMutant: false},
		{DNA: "ATGC\nGCAT\nTACG\nCGTA", IsMutant: false}, // duplicate
	} {
		if _, err := first.Save(rec); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	first.Close()

	second, err := New(path)
	if err != nil {
		t.Fatalf("New(reopen): %v", err)
	}
	defer second.Close()

	if m, h := mustStats(t, second); m != 1 || h != 1 {
		t.Fatalf("reopened stats = (%d, %d), want (1, 1)", m, h)
	}
}

func TestNewErrorOnUnwritablePath(t *testing.T) {
	// A path inside a directory that does not exist cannot be opened.
	bad := filepath.Join(t.TempDir(), "no-such-dir", "mutant.db")
	if _, err := New(bad); err == nil {
		t.Fatal("New with unwritable path returned nil error, want failure")
	}
}
