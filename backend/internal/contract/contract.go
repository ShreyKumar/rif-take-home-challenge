// Package contract defines the shared seams between the algorithm, storage,
// and HTTP layers. It holds only types (no logic) so that the handler phases
// (P3/P4) can build against these abstractions with fakes, in parallel with
// the real algorithm (P1) and store (P2).
package contract

// Record is a single de-duplicated DNA entry as persisted by a Store.
type Record struct {
	// Hash is the dedup key — the SHA-256 of the normalized DNA sequence.
	Hash string
	// DNA is the sequence with rows joined by a delimiter.
	DNA string
	// IsMutant is the cached detection result, so stats need no recomputation.
	IsMutant bool
}

// Store persists de-duplicated DNA records and exposes O(1) usage counters.
// Implementations must be safe under concurrent duplicate submissions
// (a race on the same DNA still yields exactly one row).
type Store interface {
	// Save persists rec. inserted reports whether rec was newly added
	// (false when the DNA was already stored). Counters bump only on insert.
	Save(rec Record) (inserted bool, err error)
	// Stats returns the counts of distinct stored mutant and human DNAs.
	Stats() (mutant uint64, human uint64, err error)
}

// Detector reports whether the given N×N DNA matrix belongs to a mutant.
// The real implementation is provided by the mutant package (P1).
type Detector func(dna []string) bool

// MutantRequest is the request body for POST /mutant/.
type MutantRequest struct {
	DNA []string `json:"dna"`
}

// MutantResponse is the small JSON body returned by POST /mutant/;
// the HTTP status code remains the authoritative contract.
type MutantResponse struct {
	Mutant bool `json:"mutant"`
}

// StatsResponse is the JSON body returned by GET /stats/.
type StatsResponse struct {
	CountMutantDNA uint64  `json:"count_mutant_dna"`
	CountHumanDNA  uint64  `json:"count_human_dna"`
	Ratio          float64 `json:"ratio"`
}

// ErrorResponse is the JSON body returned for error responses (e.g. 400).
type ErrorResponse struct {
	Error string `json:"error"`
}
