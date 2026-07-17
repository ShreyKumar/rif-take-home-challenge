# Code Review — RIF Mutant Detector

- **Branch / commit:** `main` @ `bb3279a` (Merge PR #20, phase-10-loadtest)
- **Date:** 2026-07-09
- **Scope:** entire repository — backend Go module, frontend, load-test harness, Dockerfile, CI/agent workflows, Makefile, git hooks, and all documentation checked against the code.
- **Method:** full manual read of every tracked file, plus a 9-dimension multi-agent review panel (algorithm, storage/concurrency, HTTP semantics, security, frontend, load-test, docs-consistency, CI/Docker, live runtime probes). 61 raw findings were deduplicated to 49 and each was adversarially verified by three independent reviewers (technical refuter, spec/scope refuter, impact assessor); 48 survived, 1 was rejected. Several findings were reproduced empirically against a running server.
- **Scope note:** findings relating to the technical-decisions document were initially excluded per request, then reviewed in a follow-up pass — see the **Addendum** at the end of this file. `TECHNICAL_DECISIONS.md` existed only as an *uncommitted, untracked* working-tree file (with a matching uncommitted README link) at review time, not on `main` itself. *(Update 2026-07-17: since committed to main via PR #28 — TD1 resolved.)*
- **No code was changed.** This file is the only addition made by the review.

---

## Executive summary

The graded core of this project is in excellent shape. The algorithm is correct (adversarially probed with hand-constructed matrices — forward-only enumeration counts each 4-window exactly once, the anti-diagonal is covered, ragged input never panics), dedup is race-safe, the status-code contract is implemented and pinned by tests, and the suite passes under `-race` with 95.8 % coverage over `internal/`. `go vet`, `gofmt` (backend), and the build are all clean.

The findings concentrate in three rings around that core:

1. **Docs vs. code (highest impact):** `requirements.md` on main describes a strict-TypeScript frontend that does not exist on main — the shipped frontend is committed vanilla JS, and README simultaneously says "vanilla JS". For a take-home where the spec doc is part of the deliverable, this is the most conspicuous defect a grader would find.
2. **Operational hardening:** no `http.Server` timeouts, an end-of-life Docker base image, ungated comment-triggered CI agent workflows with write permissions.
3. **Tooling correctness:** the k6 script's failure threshold is guaranteed to trip on a healthy server; the load-test wrapper destroys its only diagnostic log on failure.

Nothing found affects the correctness of `IsMutant`, the dedup guarantees, or the `/mutant/`–`/stats/` contract in normal operation.

### Verified as working (runtime evidence)

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l` (backend) | clean |
| `go test -race -covermode=atomic -coverpkg=./internal/... ./...` | all pass |
| Coverage (gate's denominator, `./internal/...`) | **95.8 %** (84.6 % if `cmd/server` were included) |
| Reference vectors (PDF mutant/human), overlap, all 4 orientations, ragged input | correct |
| Concurrent duplicate `Save` (64 goroutines) | exactly 1 row, 1 counter bump |
| README API examples (status codes, error strings, `"ratio":0.4`/`1` JSON rendering) | accurate |
| `loadtest/RESULTS.md` environment claims (Go version, hardware, harness description) | accurate |
| FileServer path-traversal probes (`/../backend/go.mod` etc.) | safely rejected |
| Frontend XSS surface | none (`textContent` only, no `innerHTML`/`eval`) |

---

## Findings index

| ID | Sev | Location | Title |
|---|---|---|---|
| H1 | High | `requirements.md`, `frontend/`, `README.md` | Spec declares a TypeScript frontend; main ships committed vanilla JS — three docs disagree |
| M1 | Medium | `backend/cmd/server/main.go:28` | `http.Server` has no timeouts (slowloris / idle-connection exhaustion) |
| M2 | Medium | `loadtest/mutant.js:26` | k6 `http_req_failed` threshold fails every run — 403s counted as failures |
| M3 | Medium | `.github/workflows/ci.yml:3` | "Coverage gate on every PR" claim contradicted by path filters (and already skipped for 4 merged PRs) |
| M4 | Medium | `.github/workflows/claude.yml` | Comment-triggered agent workflow: `contents:write`, no job-level gate, tag-pinned action holding the OAuth secret |
| M5 | Medium | `.github/workflows/claude-code-review.yml:42` | `allowed_bots: '*'` disables the bot gate; unfiltered PR trigger, no concurrency group |
| M6 | Medium | `backend/Dockerfile:18` | Runtime base image `alpine:3.20` is past end-of-life |
| M7 | Medium | `loadtest/run.sh:15` | EXIT trap deletes `server.log`, making startup failures undiagnosable |
| L1 | Low | `backend/internal/server/server.go:33` | Non-GET `/healthz` falls through to the file server → 404, contradicting the stated routing scheme |
| L2 | Low | `backend/internal/server/server.go:34` | Subtree patterns make `/mutant/anything` and `/stats/anything` working, persisting aliases |
| L3 | Low | `backend/internal/api/mutant.go` | `Content-Type` never validated on `POST /mutant/` (FR-2.1) |
| L4 | Low | `backend/cmd/server/main.go:39` | `log.Fatalf` skips `defer closer.Close()` on both error paths |
| L5 | Low | `backend/internal/contract/contract.go:10` | `Record.Hash` is dead — documented as the dedup key, never populated or read |
| L6 | Low | `backend/internal/server/server.go:36` | `http.FileServer(http.Dir(...))` serves dotfiles and directory listings for anything placed in `FRONTEND_DIR` |
| L7 | Low | `backend/Dockerfile:15` | No `.dockerignore` — `.git`, the challenge PDF, and stray local files (e.g. `backend/mutant.db`) enter the build context/layers |
| L8 | Low | `backend/Dockerfile:30` | `VOLUME /app/data` + README's `docker run --rm` silently discards all data |
| L9 | Low | `backend/Dockerfile:8` | Image builds with Go 1.26 while CI tests exclusively on Go 1.25 |
| L10 | Low | `backend/Dockerfile` | No `HEALTHCHECK` despite the server exposing `/healthz` |
| L11 | Low | `.github/workflows/ci.yml:30` | `setup-go` cache disabled — modernc.org/sqlite recompiled from scratch every run |
| L12 | Low | `loadtest/loadgen/main.go:82` | Per-level HTTP transports never closed; idle connections accumulate across levels |
| L13 | Low | `loadtest/loadgen/main.go:133` | req/s divides by nominal duration; failed-request latencies folded into percentiles |
| L14 | Low | `loadtest/loadgen/main.go:151` | Documented request mix is wrong: actual is 50/40/10, not ~45/45/10 |
| L15 | Low | `loadtest/RESULTS.md:61` | "Correctness holds under load" overclaims — harness only distinguishes 5xx/transport errors |
| L16 | Low | `loadtest/loadgen/main.go` | File is not gofmt-formatted, and no CI check can catch it |
| L17 | Low | `requirements.md:21` | §1 says Go "≥ 1.22" suffices; `go.mod` requires 1.25.0 |
| L18 | Low | `requirements.md:210` | §9 claims "exactly two code folders" and puts static serving in `internal/api` — both stale |
| L19 | Low | `frontend/app.js:27` | `parseDna` silently rewrites input (trim / uppercase / drop blank lines) — server errors can contradict what the user sees |
| L20 | Low | `frontend/app.js:104` | Overlapping `loadStats` calls race — a stale response can overwrite fresher stats |
| L21 | Low | `frontend/app.js:133` | Minor UI robustness: missing stats key renders `"undefined"`; disabling the focused button drops keyboard focus |
| I1–I9 | Info | various | Observations & documented trade-offs worth recording (see below) |

---

## High

### H1 — Spec declares a TypeScript frontend; main ships committed vanilla JS — three docs disagree

**Where:** `requirements.md:7-9, 25, 91-93 (FR-5.7), 190-195 (§7 item 8), 218-220 (§9), ~250 (§10 diagram)` · `frontend/app.js` · `README.md:42`

`requirements.md` on main states in at least five places that the frontend is authored in strict-mode TypeScript (`frontend/src/app.ts`, `strict: true`) compiled by `tsc`, and that `frontend/app.js` is "a generated build artifact (gitignored, not committed)" which "no longer exists in git" (§7 item 8). On main, none of that is true:

- `frontend/app.js` **is tracked in git** (`git ls-files frontend/` lists it) and is hand-written vanilla JS.
- There is no `frontend/src/`, no `app.ts`, no `tsconfig.json`.
- `README.md:42` simultaneously describes the frontend as "static HTML + vanilla JS + hand-written CSS (no build step)" — directly contradicting `requirements.md` §1/FR-5.7 in the same tree.
- §7 item 8 cross-references "plan.md Phase 11" for the TypeScript build step, but plan.md's Phase 11 is the Render deployment and says nothing about TypeScript.

By the letter of the spec, **FR-5.7 is violated on main**. In practice this looks like a docs-ahead-of-code problem: the requirements were amended for a TypeScript migration that lives on another branch, and the amendment landed on main before the code did. Either way, the first document a grader reads makes claims the repo it sits in disproves, and the two top-level docs disagree with each other.

**Fix:** either revert the `requirements.md` §1/FR-5.7/§7-8/§9/§10 wording on main to vanilla JS until the TS migration merges, or land the migration. Whichever way, make README and requirements agree, and fix the plan.md Phase 11 cross-reference.

---

## Medium

### M1 — `http.Server` has no timeouts

**Where:** `backend/cmd/server/main.go:28-31`

`srv := &http.Server{Addr: ..., Handler: handler}` sets no `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`; Go's defaults are unlimited. A client can trickle header bytes indefinitely (slowloris) or hold keep-alive connections forever, pinning a goroutine and file descriptor each, until the process FD limit starves legitimate clients. This sits oddly beside the codebase's own DoS stance: `api/mutant.go:21-25` documents the 1 MiB body cap as "the unbounded-body / compute DoS guard for NFR-2 burst traffic" — the *body* is bounded while the cheaper header/idle surface is not. (Rate limiting is out of scope per §8; basic server timeouts are standard hardening, not a rate limiter.)

**Fix:** set at minimum `ReadHeaderTimeout` (e.g. 5s), ideally also `ReadTimeout`/`WriteTimeout`/`IdleTimeout`.

### M2 — k6 script's failure threshold trips on every run against a healthy server

**Where:** `loadtest/mutant.js:26-30`

The threshold `http_req_failed: ["rate<0.01"]` uses k6's built-in failure metric, which by default counts any status outside 200–399 as failed. This workload sends the HUMAN payload on 4 of every 10 iterations, and the backend correctly answers **403** for humans — so `http_req_failed` lands at ~0.40 and k6 exits non-zero (thresholds crossed) despite a perfectly healthy server. The comment on line 27 ("mutant=200 and human=403 are both expected, non-error answers") states the intent, but that intent is only implemented in the `check` on line 40 — k6 `check`s do not feed `http_req_failed`, and the script never calls `http.setResponseCallback`.

**Fix:** add `http.setResponseCallback(http.expectedStatuses(200, 403));` at module init (matches loadgen's error definition), or replace the threshold with one on the `checks` metric (e.g. `checks: ["rate>0.99"]`).

### M3 — "Coverage gate enforced on every PR" is contradicted by the CI path filters

**Where:** `.github/workflows/ci.yml:3-13` vs `README.md:144`, `requirements.md:160-161 (NFR-3)`, `plan.md:11-12, 74-76`, `RUBRIC.md:67-68`

Four documents claim the ≥80 % gate runs on **every** PR, but the workflow only triggers on PRs touching `backend/**` or the workflow file. This is not hypothetical: merged PRs #2 (frontend), #17 (frontend fix), #20 (loadtest + Makefile), and #22 (docs/Makefile) contained no `backend/**` changes, so the "Test and coverage gate" job never ran on them. Worse, `plan.md:74-76` instructs marking that job a *required* branch-protection check — combined with the path filter, non-backend PRs would sit on a permanently "Expected" check (or, if it isn't marked required, the gate isn't actually enforced — contradicting the same paragraph).

**Fix:** drop the `paths:` filter (the job is cheap), or soften the four docs to "every PR that touches the backend" and don't mark the check required.

### M4 — Comment-triggered agent workflow runs with `contents: write` and the OAuth secret, with no workflow-level gating

**Where:** `.github/workflows/claude.yml`

The job fires on **every** `issue_comment` / `pull_request_review_comment` created event — no `if: contains(github.event.comment.body, '@claude')` gate (the official template has one). Every comment spins up a runner that checks out full history (`fetch-depth: 0`) and hands `CLAUDE_CODE_OAUTH_TOKEN` to a third-party action pinned only to the mutable `@v1` tag, under `contents: write`, `pull-requests: write`, `issues: write`, `id-token: write`. Trigger-phrase and actor enforcement live entirely inside the action. Comment bodies are attacker-controlled input to an agent holding a token that can push commits — mitigated today because the repo is private (verified), but this becomes a real prompt-injection / secret-exposure surface the moment it goes public.

**Fix:** add the standard job-level `if:` gate, drop permissions to the minimum actually used, set `fetch-depth: 1`, and pin the action to a commit SHA. Re-audit before ever making the repo public. (Also applies to the `@v4`/`@v5` tag pins in `ci.yml`.)

### M5 — `allowed_bots: '*'` disables the bot gate on the automated review workflow

**Where:** `.github/workflows/claude-code-review.yml:42` (plus 3-5, 22-26, 38)

`claude-code-action` blocks bot actors by default; the wildcard removes that gate, so any GitHub App or bot that can open a same-repo PR triggers an unattended Claude run whose prompt input is PR-controlled data (title/body/diff) — the standard LLM-CI injection surface — with the OAuth token and `id-token: write` in scope. Secondary issues: on a public repo, fork PRs would get a guaranteed-failing red check (secrets are withheld from fork `pull_request` runs), and there is no `concurrency` group, so rapid pushes run overlapping reviews of stale commits.

**Fix:** remove `allowed_bots: '*'` (or restrict to named bots), gate on `github.event.pull_request.head.repo.full_name == github.repository`, and add a `concurrency` group keyed on the PR with `cancel-in-progress: true`.

### M6 — Docker runtime base `alpine:3.20` is past end-of-life

**Where:** `backend/Dockerfile:18`

Alpine 3.20 reached end of support on 2026-04-01; its package repos are frozen, so any musl/busybox/SSL CVE published since ships unpatched in every rebuild. This undercuts otherwise good image hygiene (multi-stage, static binary, dedicated non-root uid-10001 user, scoped chown). Since `CGO_ENABLED=0` already yields a fully static binary — the Dockerfile's own stated rationale — the image doesn't need an Alpine userland at all.

**Fix:** bump to a supported Alpine (3.22+), or better `FROM scratch` / distroless-static.

### M7 — `run.sh` destroys its only diagnostic log on failure

**Where:** `loadtest/run.sh:15-19, 28`

The server's output goes to `$TMP/server.log`, and the EXIT trap unconditionally `rm -rf "$TMP"` — success or failure alike. Reproduced: with the port already occupied, the script prints only `server not ready … timed out after 10s` and exits 1; the actual bind error was in the log the trap had just deleted. The script also never checks the server process is alive after launch (`kill -0`), so an instantly-dead server still costs the full 10 s health-poll timeout, and cleanup `kill`s without `wait` before removing the directory.

**Fix:** capture `$?` at trap entry and `cat server.log` on non-zero status before removing `$TMP`; fail fast if the server PID is dead.

---

## Low

### L1 — Non-GET `/healthz` returns 404 via the file server, contradicting the stated routing scheme
`backend/internal/server/server.go:33-36`. `Assemble`'s comment explains the API handlers are deliberately registered *without* method constraints so mismatches get "405 + Allow, rather than falling through to the file server and 404-ing" — but `/healthz` is registered in the other style (`GET /healthz`). Because the unconstrained `/` pattern matches every method, ServeMux never emits its automatic 405: verified `POST /healthz` → `404 page not found`, no `Allow` header. Harmless for a liveness probe; inconsistent with the file's own design note.

### L2 — Every path under `/mutant/` and `/stats/` is a working, persisting alias
`server.go:34-35`. Trailing-slash patterns are subtree matches. Verified live: `POST /mutant/extra/path` → 200 and **persists**; `GET /stats/extra` → 200. The API contract (requirements §3) defines exactly `/mutant/` and `/stats/`; typo'd client paths silently succeed, and scanners hitting sub-paths mutate the database. Also worth documenting: `POST /mutant` (no slash) gets ServeMux's automatic 301 to `/mutant/`, which many clients re-issue as GET (→ 405) or refuse to follow with a body. Consider exact patterns (`"POST /mutant/{$}"`-style) or an explicit path check in the handlers.

### L3 — `Content-Type` never validated on `POST /mutant/`
`api/mutant.go`. FR-2.1 says the endpoint accepts `Content-Type: application/json`, but the handler never looks at the header — `text/plain` or absent content types are processed identically. Lenient parsing is defensible; it just isn't the documented contract. Either enforce (415) or document the leniency.

### L4 — `log.Fatalf` skips the deferred store close
`main.go:24, 39, 50`. `log.Fatalf` calls `os.Exit`, so `defer closer.Close()` (line 26) never runs on the listen-error or shutdown-failure paths; the goroutine at line 38 can also `Fatalf` while a graceful shutdown is mid-flight. SQLite tolerates this (WAL-less, single writer, next open recovers), so impact is negligible — but the graceful-shutdown machinery is partially bypassed by its own error handling. Return errors from a `run()` function instead of `Fatalf` in `main`.

### L5 — `contract.Record.Hash` is dead
`contract.go:9-10`. Documented as "the dedup key — the SHA-256 of the normalized DNA sequence", but the API layer never sets it and the store computes its own hash internally (`sqlite.go:91`), ignoring the field. A reader of the contract package is misled about who owns hashing. Delete the field or make the store honor it.

### L6 — FileServer serves dotfiles and directory listings from `FRONTEND_DIR`
`server.go:36`. `http.FileServer(http.Dir(dir))` auto-lists directories without `index.html` and serves any dotfile placed in the directory. Today `frontend/` contains only the three expected files (probes confirmed nothing sensitive is reachable), but the behavior is one misplaced file away from exposure — e.g. an `.env` dropped into `FRONTEND_DIR` in a deployment. A small wrapper rejecting dotfiles/listings closes it.

### L7 — No `.dockerignore`
`backend/Dockerfile:15` + build instructions ("build from the repo root"). The context ships `.git/`, the 340 KB challenge PDF, and any stray local files; `COPY backend/ ./` copies a developer's `backend/mutant.db` (present locally, gitignored but not context-ignored) into the build stage. Nothing sensitive lands in the final stage today, but the build is not reproducible across checkouts. Add a `.dockerignore` (`.git`, `*.db`, `*.pdf`, `bin/`, `coverage.out`).

### L8 — `VOLUME /app/data` + documented `docker run --rm` silently discards data
`Dockerfile:30`, `README.md:169`. `--rm` removes anonymous volumes on exit, so the documented flow provides zero persistence (POST records and `/stats/` counters reset every restart); without `--rm`, each run creates a fresh anonymous volume anyway and orphans the old one. Either drop `VOLUME` or document `-v mutant-data:/app/data`.

### L9 — Image builds on a Go toolchain CI never tests
`Dockerfile:8` (`golang:1.26-alpine`) vs `ci.yml:29` (`go-version-file: backend/go.mod` → 1.25.0). The shipped binary is compiled by a toolchain the test suite never exercises. Align the builder image with `go.mod` (or vice versa).

### L10 — No `HEALTHCHECK` despite `/healthz` existing
`Dockerfile`. One line (`HEALTHCHECK CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1`) would let orchestrators use the endpoint the server already provides. (Note the hardcoded 8080 vs `PORT` env if parameterized.)

### L11 — CI re-downloads and recompiles modernc.org/sqlite every run
`ci.yml:30` (`cache: false`). The pure-Go SQLite driver is a large compile; enabling setup-go's cache is a one-word change that meaningfully cuts CI time.

### L12 — loadgen never closes per-level transports
`loadtest/loadgen/main.go:82-88`. Each concurrency level builds a new `http.Client`/`Transport` and abandons the previous one without `CloseIdleConnections()`, so up to `2×Σlevels` idle sockets accumulate server-side across a run — mildly polluting the very measurements being taken. Close the transport at the end of `runLevel`.

### L13 — Throughput math slightly flatters high-concurrency results
`main.go:111, 121, 133, 138`. Workers check the deadline only before *starting* a request, so all `conc` in-flight requests completing after the deadline are counted, while `elapsed` stays the nominal duration; failed requests' latencies (including 10 s timeouts) are also folded into totals and percentiles. Quantified against RESULTS.md's own numbers: negligible for reads (<0.1 %), up to ~7 % inflation for mixed @ conc=128 (p99 363 ms). The "order-of-magnitude, not benchmarks" caveat keeps this honest; measure wall-clock to `wg.Wait()` to fix it.

### L14 — Request-mix comment is arithmetically wrong
`main.go:148-151`. The stats slot (`step%10 == 9`) only ever displaces odd (human) steps, so the mix is **50 % mutant / 40 % human / 10 % stats**, not "~45/45/10" as documented (verified by simulation). The mutant/human split matters when interpreting write-throughput (mutant inserts hit a different dedup branch than human). Fix the comment or the bucketing.

### L15 — RESULTS.md's "correctness holds under sustained concurrent load" overclaims
`RESULTS.md:61`, `loadgen/main.go:192`. The harness counts only transport errors and 5xx as failures (`resp.StatusCode < 500` is "success"), so a regression that returned 400 — or `{"mutant":true}` for every human — would still report "0 % errors". The evidence supports "no 5xx under load", not correctness. One-line rewording, or assert expected statuses per request type in `doRequest`.

### L16 — `loadtest/loadgen/main.go` is not gofmt-formatted
Verified: `gofmt -l loadtest/loadgen/` flags `main.go` (comment alignment in the var block at 91-95 and at 141-142). The backend module is clean, but CI runs no gofmt check and its path filter excludes `loadtest/**` anyway, so nothing can catch it. `gofmt -w` plus an optional CI fmt step.

### L17 — requirements §1 understates the toolchain floor
`requirements.md:21` says Go "requires ≥ 1.22", but `backend/go.mod` declares `go 1.25.0` — a 1.22–1.24 toolchain refuses to build (`GOTOOLCHAIN=local`). README's "Go 1.25+" is correct; requirements contradicts both.

### L18 — requirements §9 structure is stale
`requirements.md:210, 224`. "The root contains exactly two code folders — `frontend/` and `backend/`" — `loadtest/` is a third (two Go files + scripts). §9 also assigns "static serving" to `internal/api`; it actually lives in `internal/server` (`Assemble`). Minor doc drift from later phases.

### L19 — `parseDna` silently rewrites what the user typed
`frontend/app.js:27-32`. Rows are trimmed, uppercased, and blank/whitespace-only lines dropped before POSTing. Concrete confusion: type the 6-row mutant vector with row 3 as six spaces — the textarea shows a visibly square 6-line block, but the frontend sends 5 rows of width 6 and the server's 400 ("must be a square matrix") contradicts what the user sees (FR-5.4's error-surfacing intent). Similarly, lowercase input — invalid per V-3 — is silently laundered to valid uppercase rather than surfacing the server's verdict. Reasonable UX conveniences; worth either an on-screen note or dropping the silent normalization.

### L20 — Overlapping `loadStats` calls race
`app.js:104-108, 120-141, 146-151`. Three triggers can overlap (initial load, Refresh, post-submit `finally`); `refreshBtn.disabled` only guards the button path, and responses apply in arrival order. A slow Refresh response arriving after a post-submit refresh overwrites the panel with stale pre-insert counts that persist until the next manual refresh. A monotonic sequence token (apply only the latest) fixes it in three lines.

### L21 — Minor UI robustness nits
`app.js:133-135`: `String(data.count_mutant_dna)` renders the literal string `undefined` if a key is ever absent — unlike `formatRatio`, which guards with `isFinite` and shows `—`. `app.js:72`: disabling the focused submit button drops keyboard focus to `<body>`, so a keyboard user must Tab back after each check (`aria-live` on the result region does still announce the outcome).

---

## Info — observations & documented trade-offs worth recording

**I1 · `IsMutant`'s size guard assumes square input.** `mutant.go:32-34` short-circuits on `len(dna) < 4` (row count only), so a hypothetical 3×100 matrix full of horizontal runs returns `false`, and the doc comment's "any matrix smaller than 4×4" overstates the check. Unreachable in the service — `validateDNA` enforces V-2 (square) before `detect()` — and FR-1.2 defines the input contract as N×N, so this is a doc-precision point, not a defect (the spec-lens verifier dissented on even reporting it). Stating the N×N precondition in the package comment would settle it.

**I2 · Byte-level ASCII assumption.** `mutant.go:41, 70` compares bytes, correct for A/T/C/G and guarded by V-3 upstream; worth one comment line so the assumption is explicit for standalone users of the package.

**I3 · `Stats()` reads a torn pair.** `sqlite.go:120-122` performs two independent atomic loads (and `Save`'s insert→increment isn't atomic with them), so a snapshot can momentarily mismatch by one. Self-healing, invisible at rest; only worth knowing.

**I4 · Two deliberate HTTP strictness choices.** Oversized bodies map `*http.MaxBytesError` to **400** ("request body too large") rather than RFC 9110's 413 — defensible under FR-2.4's blanket-400 stance, and the typed-error detection is implemented correctly (verified with a 2 MiB body). `HEAD /stats/` returns 405 (exact `!= GET` check), which can surprise monitors that probe with HEAD.

**I5 · No security response headers** (nosniff, CSP, frame-ancestors, Referrer-Policy). Honest impact here is minimal — no cookies, no auth, no DOM XSS sink (`textContent` only) — but it's the standard baseline a production deployment would add at the router.

**I6 · No backpressure or per-request deadlines into the single SQLite connection.** `SetMaxOpenConns(1)` is deliberate and documented; but `Save` uses `Exec` (not `ExecContext(r.Context())`), so a sustained write flood queues goroutines+bodies without bound and client disconnects don't release queued work. NFR-2 scopes burst handling to design/docs and RESULTS.md documents the bottleneck honestly, so this is an observation; threading the request context through would be cheap hardening.

**I7 · The enforced coverage number is `internal/`-only.** `-coverpkg=./internal/...` excludes `cmd/server` (52 lines, 0 % — `main` is untested) from the denominator: 95.8 % as gated vs 84.6 % over `./...`. Passes either way; README's "backend coverage gate" phrasing is ~11 points softer than it sounds, while the CI step's own log says "(internal/)" honestly. A defensible, common choice — worth one documented line.

**I8 · plan.md P10 promised a k6 harness; a custom stdlib loadgen landed instead** (k6 demoted to "documented alternative"). Arguably an improvement given the dependency-free goal; the plan doc just wasn't updated to match.

**I9 · Multi-replica `/stats/` (design-path nuance).** The in-memory counters are per-process and seeded once at startup, so *if* multiple replicas ever shared one database, `/stats/` would diverge per replica. The adversarial panel rejected this as a defect — requirements §10.1 explicitly places "atomic counters — in-memory default / Redis scale-path" on the dashed (design-only) scale path, the "stateless" claim is qualified as per-request state, and main is single-instance by design. Recorded here only as an operational caveat: don't run N replicas against a shared DB without also moving the counters, which the docs already anticipate.

---

## Considered and rejected

- **"In-memory counters contradict the stateless/horizontal-scale claim" (as a defect):** killed in verification — the docs qualify the claim precisely and place counter externalization on the documented Redis scale-path (see I9 for the retained nuance).
- **`ratio = 0.0` with zero humans, uppercase-only alphabet, 400-for-invalid, overlap counting:** all flagged by individual reviewers and all dismissed — each is an explicitly documented decision (requirements §7) implemented exactly as documented and pinned by tests.
- **Missing auth / rate limiting / real load infra:** out of scope per requirements §8; not reported.

## Strengths worth keeping

- **Algorithm:** clean forward-only window enumeration (each window counted exactly once, both diagonals covered), true early-exit, bounds-safe on ragged input; the test table pins each orientation *in isolation*, the overlap convention, and the FR-1.4 threshold — including a deliberately asymmetric reseed fixture (`sqlite_test.go:125-128`) that would catch transposed counters.
- **Layering:** `contract` as a dependency seam genuinely decouples handlers from algorithm/store (handlers are tested with fakes; `server.Assemble` is a pure composition root).
- **Store:** `UNIQUE(dna_hash)` + `ON CONFLICT DO NOTHING` + `RowsAffected` is the right idempotency mechanism, and the 64-goroutine race test proves it under `-race`.
- **Tests:** behavior-first assertions throughout (raw JSON wire-shape test, `Allow`-header checks, invalid-input-not-persisted E2E, restart-persistence E2E). 95.8 % coverage with no padding detected.
- **Docs discipline:** requirement IDs traced in code comments; RESULTS.md's honest single-node caveats; documented decisions (§7) genuinely match implemented behavior (H1 aside).
- **Ops basics:** multi-stage static build, non-root runtime user, graceful shutdown on SIGINT/SIGTERM, env-var config with sane defaults, PR-only main via pre-push hook.

## Suggested fix order

1. **H1** — align requirements.md/README on the frontend story for main (docs-only change, biggest grader-facing impact).
2. **M1** (`ReadHeaderTimeout` etc.) and **M2** (k6 response callback) — two-line fixes.
3. **M4/M5** — gate the agent workflows and trim permissions before the repo is ever public.
4. **M6/M7** — bump the base image; dump `server.log` on failure.
5. **M3** + the L-tier doc drifts (L14, L15, L17, L18) in one docs-sweep PR; L16 (`gofmt -w`) alongside. Fold the Addendum's TD findings (below) into the same docs sweep — TD1 (commit the file) and TD2/TD3 (correct the frontend and workflow claims) before anything else references the doc.
6. Remaining L items opportunistically — none are urgent.

---

# Addendum — `TECHNICAL_DECISIONS.md` review

Reviewed in a follow-up pass. The file (150 lines, 12 sections) exists **only as an untracked working-tree file** (mtime 2026-07-09 13:55), together with an uncommitted `README.md` edit that adds the link to it; committed `main` contains neither. It was not created by this review. Claims were checked against the code, the other docs, and — for §12 — the repository's own git history.

## Addendum findings

### TD1 (Medium) — The doc and its README link are uncommitted, so the deliverable doesn't ship *(resolved 2026-07-17 by PR #28)*

`git status`: `?? TECHNICAL_DECISIONS.md`, ` M README.md`. On committed `main` — i.e. what a grader clones — the doc does not exist. Once committed, the README link resolves (the main review's earlier broken-link concern disappears); until then, this entire document is invisible outside this machine. **Fix:** commit both files together (via the usual PR flow — note the pre-push hook blocks direct pushes to `main`).

### TD2 (Medium — deepens H1) — §4 takes the *opposite* side of the frontend-language contradiction

`TECHNICAL_DECISIONS.md:46-50` presents "Why Vanilla JS?" as the standing decision and argues explicitly **against** a TypeScript step ("remove any intermediary players (Typescript compilers, React Virtual DOM etc)"). `requirements.md` §7 item 8 / FR-5.7 record the **revised** decision *to* strict-mode TypeScript. The doc corpus now holds two positions: requirements = TypeScript vs. README + TECHNICAL_DECISIONS = vanilla JS. Whichever is intended, §4 needs either the revision note (mirroring requirements §7-8) or requirements needs reverting — H1 and TD2 must be fixed as one coherent story.

### TD3 (Medium) — §12's workflow claims are factually wrong, and invert the security posture

`TECHNICAL_DECISIONS.md:142-150` makes three checkable claims; two fail:

1. *"The assistant workflow was simplified down from the default scaffold (narrower trigger set, **narrower permissions**)"* — the trigger set did narrow (4 events → 2), but git history shows the opposite for everything else. The original scaffold (`5b68dc5`) had a job-level `if: contains(..., '@claude')` gate, **read** permissions (`contents/pull-requests/issues: read`), and `fetch-depth: 1`. The "simplify" commit (`17a2f5d`) **deleted the trigger-phrase gate, escalated all three permissions to `write`, and deepened checkout to `fetch-depth: 0`** — precisely the exposures flagged as M4. The doc describes a hardening; the diff shows a loosening.
2. *"**Both** workflows are pinned to `claude-haiku-4-5-20251001`"* — false. The pin commit (`fbdd21b`) touched only `claude.yml`; `claude-code-review.yml` contains no model configuration and runs the action's default model.
3. *(Accurate)* the `allowed_bots: '*'` rationale ("so PRs opened by bot accounts — including Claude itself — still get reviewed") is genuinely documented here — which answers M5's *intent* question, though the wildcard remains overbroad versus naming the specific bots.

**Fix:** correct both statements (and, per M4, actually restore the gate/read-permissions the doc believes exist — at which point the sentence becomes true).

### TD4 (Low) — §9 documents a Render deployment that is not on main

`TECHNICAL_DECISIONS.md:94-105` describes `render.yaml`, `DB_PATH=/tmp/mutant.db` on Render, and a repo-root-relative `FRONTEND_DIR` — none of which exist on `main`; `render.yaml` lives on the unmerged `phase-11-render` branch. Same docs-ahead-of-code drift class as H1. Fine if the doc is committed after/with P11; wrong if it lands on main first.

### TD5 (Low) — §10 repeats the stale "exactly two top-level code folders" claim, and contradicts §7 of the same document

`TECHNICAL_DECISIONS.md:109-111` claims exactly two code folders (`frontend/`, `backend/`) as a deliberate choice, while §7 (lines 66-83) of the *same document* describes the third one (`loadtest/` — two Go files plus scripts). Mirrors L18 in `requirements.md` §9.

### TD6 (Low) — Section numbering skips §8

Sections run 1–7, then 9–12. Looks like a section was deleted without renumbering (plausibly alongside the cut P9 stress phase that §6 discusses). Cosmetic, but conspicuous in a document whose purpose is careful decision-keeping.

### TD7 (Info) — Prose accuracy and polish nits

- "SQLLite" (×3, `:19`) and "PostgresSQL" (`:19`) misspellings; "explicitely" (`:3`, `:55`).
- §1's "Unlike other programming languages, it has concurrency built in" (`:15`) overclaims — many languages have built-in concurrency; the defensible claim is about Go's cheap goroutines/scheduler.
- The narrative paragraphs (§1 "Why Go", §2 "Why SQLLite", §4) are noticeably rougher than the polished bullet sections — reads as an unedited draft pass sitting beside finished material.

## Addendum — claims verified accurate

Checked and confirmed against code/history, worth crediting:

- **§1** Go-version story is correct and *resolves* L17's ambiguity properly (≥1.22 needed for method+pattern routing; module pins 1.25.0) — unlike `requirements.md` §1, which should adopt this wording.
- **§2** storage mechanics all match `sqlite.go`: pool-of-1 rationale including the `:memory:` per-connection-isolation nuance, constraint-enforced dedup (not application logic), counters seeded once and bumped only on genuine insert.
- **§3** forward-only scan description is exactly what `mutant.go` implements (each window counted from exactly one starting cell — verified adversarially in the main review).
- **§6** the P9 stress-phase removal matches git history (`28ba93c`, `afe6d80`, `b2549d9`) and the retained concurrency coverage (`TestConcurrentDuplicateSave` under `-race`) exists as claimed.
- **§7** loadtest numbers and framing match `RESULTS.md` (~160k req/s reads, few-thousand writes, single-writer bottleneck as the empirically confirmed scale-path motivation).
- **§10** the `//go:embed` cannot-cross-module-boundary limitation and the serve-from-disk consequence are accurate.
- **§11** the scale narrative matches README/requirements, and its "stateless (no per-request state in the process)" phrasing is the properly qualified version (see I9).
