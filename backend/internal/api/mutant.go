// Package api holds the HTTP handlers for the mutant-detector service. Each
// handler is built against the contract abstractions (a Detector and a Store)
// and injected with concrete implementations centrally in P5, so the handlers
// are unit-testable with fakes and never import the algorithm or store
// packages directly.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
)

// dnaRowDelimiter joins the DNA rows into the single canonical string the
// Store hashes and persists.
const dnaRowDelimiter = "\n"

// NewMutantHandler builds the POST /mutant/ handler. It validates the payload,
// runs the injected detector, persists the result via the injected store, and
// answers 200 (mutant) / 403 (human) / 400 (invalid) / 405 (non-POST). The
// HTTP status is the authoritative contract; the JSON body is advisory (FR-2).
func NewMutantHandler(detect contract.Detector, store contract.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMutantError(w, http.StatusMethodNotAllowed, "method not allowed; use POST")
			return
		}

		var req contract.MutantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeMutantError(w, http.StatusBadRequest, "malformed JSON body")
			return
		}

		if err := validateDNA(req.DNA); err != nil {
			writeMutantError(w, http.StatusBadRequest, err.Error())
			return
		}

		isMutant := detect(req.DNA)

		// Every valid DNA is stored; the store de-duplicates (FR-3.1/3.2).
		rec := contract.Record{
			DNA:      strings.Join(req.DNA, dnaRowDelimiter),
			IsMutant: isMutant,
		}
		if _, err := store.Save(rec); err != nil {
			writeMutantError(w, http.StatusInternalServerError, "could not persist result")
			return
		}

		status := http.StatusForbidden // human
		if isMutant {
			status = http.StatusOK
		}
		writeMutantJSON(w, status, contract.MutantResponse{Mutant: isMutant})
	})
}

// validateDNA enforces the boundary rules (§4). A nil error means the matrix
// is a valid N×N grid of A/T/C/G; note N<4 is valid (never mutant), which is
// distinct from invalid input.
func validateDNA(dna []string) error {
	n := len(dna)
	if n == 0 { // V-1
		return errors.New("dna must be a non-empty array of strings")
	}
	for _, row := range dna {
		if len(row) != n { // V-2 (square)
			return errors.New("dna must be a square matrix: every row length must equal the number of rows")
		}
		for i := 0; i < len(row); i++ { // V-3 (A/T/C/G only)
			switch row[i] {
			case 'A', 'T', 'C', 'G':
			default:
				return errors.New("dna may contain only the characters A, T, C, G")
			}
		}
	}
	return nil
}

// writeMutantJSON writes body as a JSON response with the given status. The
// name is scoped to this file so it does not collide with the /stats/ handler
// added in a parallel branch (both live in package api).
func writeMutantJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeMutantError(w http.ResponseWriter, status int, msg string) {
	writeMutantJSON(w, status, contract.ErrorResponse{Error: msg})
}
