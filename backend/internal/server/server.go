// Package server is the composition root for the mutant-detector service: it
// assembles the HTTP handler from the algorithm, store, and API-handler
// packages and mounts every route. Assemble is a pure, dependency-injected
// builder (unit-testable with fakes); New wires the production dependencies
// from configuration and returns a Closer for the underlying store.
package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/api"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/config"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/contract"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/mutant"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/store"
)

// Assemble builds the root HTTP handler from injected dependencies. It performs
// no I/O and opens no database, so tests can exercise the full routing table
// with fakes or an in-memory store. It mounts:
//   - GET /healthz   liveness probe
//   - /mutant/       POST detection (the handler self-checks method → 405)
//   - /stats/        GET usage stats (the handler self-checks method → 405)
//   - /              static frontend from frontendDir via http.FileServer
//
// The endpoint handlers are registered without a method constraint so their own
// method checks emit 405 + Allow, rather than falling through to the file
// server and 404-ing.
func Assemble(detect contract.Detector, st contract.Store, frontendDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.Handle("/mutant/", api.NewMutantHandler(detect, st))
	mux.Handle("/stats/", api.NewStatsHandler(st))
	mux.Handle("/", http.FileServer(http.Dir(frontendDir)))
	return mux
}

// New opens the configured SQLite store and assembles the production handler,
// injecting mutant.IsMutant as the detector. The returned io.Closer owns the
// store and must be closed on shutdown.
func New(cfg config.Config) (http.Handler, io.Closer, error) {
	st, err := store.New(cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	return Assemble(mutant.IsMutant, st, cfg.FrontendDir), st, nil
}

// healthz is a liveness probe returning 200 with the body "ok".
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
