# RIF Mutant Detector

A Go backend plus a static frontend that detects whether a subject is a mutant from their DNA matrix.

See [requirements.md](./requirements.md) for the full requirements and [plan.md](./plan.md) for the phased implementation plan.

## Run (backend)

```sh
cd backend && go run ./cmd/server
```

This serves the health check at `/healthz` on port `:8080` (override with `PORT`).

The full README — build/run/test instructions, `curl` examples, and the scalability narrative — lands in Phase 8.
