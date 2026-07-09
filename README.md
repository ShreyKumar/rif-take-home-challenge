# RIF Mutant Detector

A Go backend plus a static frontend that detects whether a subject is a mutant from their DNA matrix.

See [requirements.md](./requirements.md) for the full requirements and [plan.md](./plan.md) for the phased implementation plan.

## Run

The app has two parts:

- **Backend** — a Go API server (`backend/`).
- **Frontend** — a static site (`frontend/`: `index.html`, `styles.css`, `app.js`).

The frontend talks to the API over **same-origin relative paths** (`POST /mutant/`, `GET /stats/`), so the Go server serves both the static assets and the API from a single origin — no CORS or extra config needed. The static directory is configurable via `FRONTEND_DIR` (default `../frontend`).

### Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- Python 3 or Node.js — optional, only to preview the frontend on its own

### Run both (integrated)

Start the backend from the `backend/` directory; it serves the API and the frontend at `/`:

```sh
cd backend
go run ./cmd/server
```

Then open <http://localhost:8080> — the UI, `/mutant/`, and `/stats/` are all on that one origin.

Configuration is read from the environment:

| Variable       | Default       | Purpose                                  |
| -------------- | ------------- | ---------------------------------------- |
| `PORT`         | `8080`        | TCP port the server listens on           |
| `DB_PATH`      | `mutant.db`   | SQLite database file path                |
| `FRONTEND_DIR` | `../frontend` | Directory of static assets served at `/` |

### Frontend only (UI preview)

To iterate on the UI without the backend, serve the static files directly:

```sh
cd frontend
python3 -m http.server 5173   # then open http://localhost:5173
```

In this mode the `/mutant/` and `/stats/` calls have no backend to reach, so the API-backed features won't work — use the integrated command above for the full flow.

> **Status:** the server currently exposes `/healthz` on `:8080`; static-file serving at `/` and the `/mutant/` and `/stats/` endpoints are mounted in later phases (see [plan.md](./plan.md)). The commands above describe how each component is run. The full README — build/test instructions, `curl` examples, and the scalability narrative — lands in Phase 8.
