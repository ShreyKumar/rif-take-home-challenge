# Load-test results

Throughput / latency evidence for the mutant-detector service, captured with
the in-repo [`loadgen`](./loadgen) driver via `make loadtest`.

> **Honest caveat.** These numbers are from a **single developer machine** over
> the **loopback interface**. The brief's 100–1M req/s figure is a design target,
> not something a single node measures; the unbuilt horizontal-scale path is
> future work (see [../TECHNICAL_DECISIONS.md](../TECHNICAL_DECISIONS.md) §9).
> Numbers vary run to run; treat them as order-of-magnitude, not benchmarks.

## Environment

| | |
|---|---|
| Hardware | Apple M4 Max, 16 cores |
| OS | macOS (Darwin 25.3.0) |
| Go | go1.26.5 |
| Server | `backend/cmd/server`, default config, file-backed SQLite |
| Client | `loadtest/loadgen`, loopback, HTTP keep-alive, 5s per level |

## Reads — `GET /stats/` (O(1) maintained counters)

The stats path reads in-memory atomic counters, so it scales with cores and
stays flat on latency — evidence for **FR-4.5 / NFR-2** (O(1) stats, no table
scan).

| concurrency | req/s | p50 | p99 | errors |
|---:|---:|---:|---:|---:|
| 1 | 28,283 | 0.03 ms | 0.07 ms | 0% |
| 8 | 98,383 | 0.07 ms | 0.24 ms | 0% |
| 32 | 122,211 | 0.23 ms | 0.81 ms | 0% |
| 64 | 148,310 | 0.37 ms | 1.43 ms | 0% |
| 128 | **160,266** | 0.70 ms | 2.59 ms | 0% |

## Mixed — 90% `POST /mutant/` + 10% `GET /stats/`

The write path validates → runs the detector → does a SHA-256 dedup upsert. Each
write serializes through SQLite's **single writer connection**
(`SetMaxOpenConns(1)`), so aggregate throughput plateaus at a few thousand req/s
and tail latency grows with queueing — **still 0% errors**.

| concurrency | req/s | p50 | p99 | errors |
|---:|---:|---:|---:|---:|
| 1 | 4,275 | 0.23 ms | 0.33 ms | 0% |
| 8 | 4,046 | 1.08 ms | 9.46 ms | 0% |
| 32 | 2,144 | 5.03 ms | 144.96 ms | 0% |
| 64 | 2,192 | 11.40 ms | 306.84 ms | 0% |
| 128 | 4,078 | 15.66 ms | 363.08 ms | 0% |

## Reading the results

- **Reads are effectively free** and scale to ~160k req/s on one node — the O(1)
  counter design pays off exactly where high read volume is expected.
- **Writes are bounded by the single SQLite writer**, by design for the local /
  demo configuration — an empirically confirmed bottleneck. Removing it (a
  Postgres/Redis store swap behind the same `Store` interface) is **not implemented**;
  it's recorded as future work in [../TECHNICAL_DECISIONS.md](../TECHNICAL_DECISIONS.md) §9.
- **Zero 5xx** at every level. The harness only distinguishes transport errors and
  HTTP ≥ 500, so this evidences *"no 5xx under sustained load"* — not full functional
  correctness.

## Reproduce

```sh
make loadtest                      # reads + mixed, default levels
# or, against an already-running server:
cd loadtest/loadgen && go build -o /tmp/loadgen .
/tmp/loadgen -url http://127.0.0.1:8080 -mix reads  -levels 1,8,32,64,128 -duration 5s
/tmp/loadgen -url http://127.0.0.1:8080 -mix mixed  -levels 1,8,32,64,128 -duration 5s
```
