// Package store provides the SQLite-backed implementation of contract.Store.
//
// De-duplication is enforced by a UNIQUE index on the SHA-256 hash of the DNA
// sequence, combined with INSERT … ON CONFLICT DO NOTHING, so concurrent
// duplicate submissions still yield exactly one row (FR-3). Usage counts are
// served in O(1) from in-memory atomic counters seeded once at startup, so
// GET /stats/ never scans the table (FR-4.5 / NFR-2).
package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// schema is the single de-duplicated records table (requirements §5).
const schema = `
CREATE TABLE IF NOT EXISTS dna_records (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	dna_hash   TEXT    NOT NULL UNIQUE,
	dna        TEXT    NOT NULL,
	is_mutant  INTEGER NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// SQLiteStore implements contract.Store over an embedded SQLite database.
type SQLiteStore struct {
	db     *sql.DB
	mutant atomic.Uint64
	human  atomic.Uint64
}

// compile-time assertion that *SQLiteStore satisfies the shared interface.
var _ contract.Store = (*SQLiteStore)(nil)

// New opens (or creates) the SQLite database at path, ensures the schema
// exists, and seeds the in-memory counters from the current row counts.
//
// The pool is pinned to a single connection: SQLite serialises writers anyway,
// this avoids SQLITE_BUSY under concurrent Saves, and it keeps an in-memory
// (":memory:") database — where each connection is otherwise isolated — usable
// in tests.
func New(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.seedCounters(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// seedCounters loads the mutant/human counts once so /stats/ is O(1) and
// counts survive a restart against an existing database file.
func (s *SQLiteStore) seedCounters() error {
	const q = `
SELECT
	COALESCE(SUM(CASE WHEN is_mutant = 1 THEN 1 ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN is_mutant = 0 THEN 1 ELSE 0 END), 0)
FROM dna_records;`
	var mutant, human uint64
	if err := s.db.QueryRow(q).Scan(&mutant, &human); err != nil {
		return fmt.Errorf("seed counters: %w", err)
	}
	s.mutant.Store(mutant)
	s.human.Store(human)
	return nil
}

// Save persists rec, keyed by the SHA-256 of its DNA. It reports whether the
// row was newly inserted; a duplicate returns inserted=false and leaves the
// counters untouched. The UNIQUE + ON CONFLICT DO NOTHING pair makes this safe
// under concurrent duplicate submissions (FR-3.2, FR-3.3).
func (s *SQLiteStore) Save(rec contract.Record) (inserted bool, err error) {
	hash := hashDNA(rec.DNA)

	res, err := s.db.Exec(
		`INSERT INTO dna_records (dna_hash, dna, is_mutant) VALUES (?, ?, ?)
		 ON CONFLICT(dna_hash) DO NOTHING;`,
		hash, rec.DNA, boolToInt(rec.IsMutant),
	)
	if err != nil {
		return false, fmt.Errorf("insert record: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return false, nil // duplicate DNA — already stored
	}

	if rec.IsMutant {
		s.mutant.Add(1)
	} else {
		s.human.Add(1)
	}
	return true, nil
}

// Stats returns the distinct mutant and human counts in O(1) from the
// maintained counters — no table scan (FR-4.5).
func (s *SQLiteStore) Stats() (mutant uint64, human uint64, err error) {
	return s.mutant.Load(), s.human.Load(), nil
}

// Close releases the underlying database handle.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// hashDNA returns the hex-encoded SHA-256 of the caller-supplied DNA string,
// used as the dedup key. It hashes the argument as-is and does not itself
// normalize; callers pass the already-canonical sequence (the API layer
// validates uppercase A/T/C/G and joins rows with a fixed delimiter upstream).
func hashDNA(dna string) string {
	sum := sha256.Sum256([]byte(dna))
	return hex.EncodeToString(sum[:])
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
