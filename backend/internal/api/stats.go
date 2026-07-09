package api

import (
	"encoding/json"
	"net/http"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
)

// NewStatsHandler builds the GET /stats/ handler over an injected Store. It
// reads the maintained O(1) counters and returns
// {count_mutant_dna, count_human_dna, ratio}, where ratio = mutant / human
// (0.0 when there are zero humans, FR-4.4). Non-GET requests get 405 (FR-4).
func NewStatsHandler(store contract.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeStatsError(w, http.StatusMethodNotAllowed, "method not allowed; use GET")
			return
		}

		mutant, human, err := store.Stats()
		if err != nil {
			writeStatsError(w, http.StatusInternalServerError, "could not read stats")
			return
		}

		ratio := 0.0
		if human > 0 { // FR-4.4: avoid divide-by-zero
			ratio = float64(mutant) / float64(human)
		}

		writeStatsJSON(w, http.StatusOK, contract.StatsResponse{
			CountMutantDNA: mutant,
			CountHumanDNA:  human,
			Ratio:          ratio,
		})
	})
}

// writeStatsJSON writes body as a JSON response with the given status. The
// name is scoped to this file so it does not collide with the /mutant/
// handler's helper in the parallel branch (both live in package api).
func writeStatsJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeStatsError(w http.ResponseWriter, status int, msg string) {
	writeStatsJSON(w, status, contract.ErrorResponse{Error: msg})
}
