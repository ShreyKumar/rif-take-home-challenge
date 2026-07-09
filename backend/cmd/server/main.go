// Command server is the thin bootstrap for the mutant-detector service:
// it loads configuration, builds the handler, and runs the HTTP server with
// graceful shutdown. All business logic lives in tested internal/ packages.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/config"
	"github.com/ShreyKumar/rif-take-home-challenge/backend/internal/server"
)

func main() {
	cfg := config.Load()

	handler, closer, err := server.New(cfg)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}
	defer closer.Close()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
}
