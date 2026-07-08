// Package server builds the HTTP handler for the mutant-detector service.
// Phase 0 registers only a health check; later phases extend New() to mount
// the /mutant/, /stats/, and static-file routes.
package server

import "net/http"

// New builds the root HTTP handler.
func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	return mux
}

// healthz is a liveness probe returning 200 with the body "ok".
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
