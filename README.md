# RIF Mutant Detector

A Go backend plus a static frontend that detects whether a subject is a mutant
from their DNA matrix, exposes a small REST API, persists de-duplicated results
in SQLite, and reports usage statistics.

- **Algorithm** — `IsMutant(dna)`: an O(N²) single-pass scan for four-in-a-row
  sequences across all four orientations, with early-exit.
- **API** — `POST /mutant/` and `GET /stats/`, plus the static UI at `/`.
- **Storage** — embedded SQLite behind a `Store` interface, one row per unique
  DNA, with O(1) usage counters.

See [requirements.md](./requirements.md) for the full spec (and the architecture
diagram in §10) and [plan.md](./plan.md) for the phased delivery.

## Architecture

```
Browser ──HTTP──> Go server (stateless) ──> Store interface ──> SQLite (UNIQUE dna_hash)
  static UI          ├─ GET  /          FileServer(frontend/)      + in-memory O(1)
  fetch()            ├─ POST /mutant/    validate → detect → save     atomic counters
                     └─ GET  /stats/     read counters (O(1))
```

The frontend calls the API over **same-origin relative paths**, so a single Go
process serves both the static assets and the API — no CORS, no second server.
Full component and sequence diagrams are in [requirements.md §10](./requirements.md#10-architecture).

## Project structure

```
backend/           self-contained Go module (backend/go.mod)
  cmd/server/      main(): config → assemble → ListenAndServe + graceful shutdown
  internal/mutant/ IsMutant algorithm
  internal/api/    POST /mutant/ and GET /stats/ handlers
  internal/store/  Store interface + SQLite impl (dedup + counters)
  internal/server/ composition root: assembles handlers + mounts routes
  internal/config/ env-var configuration
  internal/contract/ shared types (Store, Detector, DTOs)
frontend/          static HTML + vanilla JS + hand-written CSS (no build step)
loadtest/          dependency-free load harness + measured results
Makefile           build / run / test / loadtest
```

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- Optional: Docker (to run the containerized build)

## Build & run

The Go server serves the API **and** the frontend at `/` from one process:

```sh
cd backend
go run ./cmd/server
# → listening on :8080
```

Open <http://localhost:8080> — the UI, `POST /mutant/`, and `GET /stats/` are all
on that one origin. Or use the Makefile from the repo root:

```sh
make run      # go run ./cmd/server
make build    # → backend/bin/server
```

### Configuration (environment variables)

| Variable       | Default       | Purpose                                  |
| -------------- | ------------- | ---------------------------------------- |
| `PORT`         | `8080`        | TCP port the server listens on           |
| `DB_PATH`      | `mutant.db`   | SQLite database file path                |
| `FRONTEND_DIR` | `../frontend` | Directory of static assets served at `/` |

## API

| Method | Path       | Success                    | Errors                          |
| ------ | ---------- | -------------------------- | ------------------------------- |
| `POST` | `/mutant/` | `200` mutant · `403` human | `400` invalid · `405` non-POST  |
| `GET`  | `/stats/`  | `200` + stats JSON         | `405` non-GET                   |
| `GET`  | `/`        | `200` + HTML UI            | —                               |

The **HTTP status code is the authoritative contract**; the JSON body is advisory.

### `POST /mutant/`

Mutant → `200`:

```sh
curl -i -X POST http://localhost:8080/mutant/ \
  -H 'Content-Type: application/json' \
  -d '{"dna":["ATGCGA","CAGTGC","TTATGT","AGAAGG","CCCCTA","TCACTG"]}'
# HTTP/1.1 200 OK
# {"mutant":true}
```

Human → `403`:

```sh
curl -i -X POST http://localhost:8080/mutant/ \
  -H 'Content-Type: application/json' \
  -d '{"dna":["ATGC","GCAT","TACG","CGTA"]}'
# HTTP/1.1 403 Forbidden
# {"mutant":false}
```

Invalid (non-square) → `400`; non-POST → `405` with `Allow: POST`:

```sh
curl -i -X POST http://localhost:8080/mutant/ -d '{"dna":["ATGC","CAG"]}'
# HTTP/1.1 400 Bad Request
# {"error":"dna must be a square matrix: every row length must equal the number of rows"}

curl -i http://localhost:8080/mutant/
# HTTP/1.1 405 Method Not Allowed
# Allow: POST
```

### `GET /stats/`

```sh
curl -s http://localhost:8080/stats/
# {"count_mutant_dna":1,"count_human_dna":1,"ratio":1}
```

`ratio = count_mutant_dna / count_human_dna` (matches the brief's `40/100 = 0.4`),
and is `0.0` when there are zero humans.

## Test

```sh
make test    # go test -race with coverage, prints the total
```

or directly:

```sh
cd backend && go test -race -covermode=atomic -coverpkg=./internal/... ./...
```

CI enforces a **≥ 80% backend coverage gate** on every PR that touches the backend
([.github/workflows/ci.yml](./.github/workflows/ci.yml) triggers on `backend/**`);
the suite currently sits well above it. The frontend is verified manually (no JS test suite).

## Load testing

A dependency-free Go load harness lives in [`loadtest/`](./loadtest):

```sh
make loadtest    # builds + starts the server, drives reads + mixed load, tears down
```

Measured numbers and the honest caveats are in
[loadtest/RESULTS.md](./loadtest/RESULTS.md): the O(1) `/stats/` read path scales
to ~160k req/s on one machine, while writes are bounded by SQLite's single writer
connection.

## Docker

A multi-stage image builds a static binary (pure-Go SQLite, no cgo) and bundles
the frontend. Build from the **repo root** (context needs both `backend/` and
`frontend/`):

```sh
docker build -f backend/Dockerfile -t mutant-detector .
docker run --rm -p 8080:8080 mutant-detector
# → http://localhost:8080
```

## Deploy (live demo)

A [`render.yaml`](./render.yaml) Blueprint deploys the assembled service to
[Render](https://render.com)'s free tier. On Render, choose **New → Blueprint**,
point it at this repo, and apply — it builds `backend/cmd/server` from the repo
root and serves the API + frontend, health-checked at `/healthz`. Render injects
`PORT`; the server reads it.

> **Live demo:** <https://mutant-detector-wc41.onrender.com> — on Render's free
> tier, so the first request after an idle period may take ~30–60s to wake.

**Trade-off (documented):** Render's free tier has **no persistent disk**, so the
SQLite database (`DB_PATH=/tmp/mutant.db`) is **ephemeral** — the stored DNAs and
`/stats/` counts reset when the instance idles out (~15 min) or redeploys. That's
fine for a public demo link and does **not** change NFR-6: the local/default run
still needs no external services and no build step beyond `go run`.

## Scalability

The service is a single stateless Go process. Two **implemented** properties keep
it efficient under load:

- **O(1) statistics.** `/stats/` reads in-memory atomic counters seeded once at
  startup — never a `COUNT(*)` scan.
- **Dedup at the storage layer.** `UNIQUE(dna_hash)` + upsert means repeated DNA
  never creates a second row or double-counts, shielding the DB from repeat writes.

Measured on one node in [loadtest/RESULTS.md](./loadtest/RESULTS.md). Horizontal
scaling (a load balancer, a Postgres/Redis store swap behind the `Store` interface)
is **not implemented** — it's recorded as future work in
[TECHNICAL_DECISIONS.md](./TECHNICAL_DECISIONS.md) §9.

## Documented decisions

Judgment calls where the brief is silent or ambiguous (full list in
[requirements.md §7](./requirements.md#7-assumptions--documented-decisions)):

1. **Overlap counting** — overlapping four-base windows count independently (a
   run of five = two sequences).
2. **`ratio = mutants ÷ humans`** (per the `40/100 = 0.4` example), not ÷ total.
3. **`400` for invalid input** — the brief only mandates 200/403 for valid DNA;
   malformed input gets 400 as sound REST practice.
4. **`ratio = 0.0` when there are zero humans.**
5. **Uppercase `A/T/C/G` only.**
6. **SQLite** as the default store, abstracted behind a `Store` interface.
7. **Small JSON body** on `/mutant/`, with the **status code authoritative**.
