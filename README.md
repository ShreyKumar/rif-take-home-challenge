# RIF Mutant Detector

A Go backend plus a static frontend that detects whether a subject is a mutant from their DNA matrix.

See [requirements.md](./requirements.md) for the full requirements and [plan.md](./plan.md) for the phased implementation plan.

## Run (backend)

```sh
cd backend && go run ./cmd/server
```

This serves the health check at `/healthz` on port `:8080` (override with `PORT`).

## Frontend

The frontend is written in TypeScript (`frontend/src/app.ts`) and compiled to plain JS
(`frontend/app.js`) via `tsc` — no framework, no bundler. The compiled `app.js` is a generated build
artifact and is not committed, so build it once before serving the frontend (standalone or via the
backend):

```sh
cd frontend
npx tsc           # compile to app.js
npx tsc --watch   # or: watch for changes
```

The full README — build/run/test instructions, `curl` examples, and the scalability narrative — lands in Phase 8.
