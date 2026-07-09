# Load testing

A small, **dependency-free** load harness for the mutant-detector service.
Because the project deliberately avoids external tooling, the primary driver is
a stdlib-only Go program ([`loadgen`](./loadgen)); a [k6](https://k6.io) script
([`mutant.js`](./mutant.js)) is included as a documented alternative for those
who already have k6.

See [RESULTS.md](./RESULTS.md) for measured numbers and the honest caveats.

## Quick start

From the repo root:

```sh
make loadtest      # builds + starts the server on a temp DB, runs reads + mixed, tears down
```

Override the defaults via env vars:

```sh
LEVELS=1,16,64,256 DURATION=10s PORT=8300 make loadtest
```

## Running `loadgen` by hand

Against an already-running server:

```sh
cd loadtest/loadgen
go build -o /tmp/loadgen .
/tmp/loadgen -url http://127.0.0.1:8080 -mix mixed -levels 1,8,32,64,128 -duration 5s
```

Flags:

| flag | default | meaning |
|---|---|---|
| `-url` | `http://127.0.0.1:8080` | base URL of the running server |
| `-levels` | `1,8,32,64,128` | comma-separated concurrency levels |
| `-duration` | `3s` | closed-loop load duration per level |
| `-mix` | `mixed` | `mixed` (90% write / 10% stats), `writes`, or `reads` |

`loadgen` waits for `/healthz` before starting, then for each level runs that many
worker goroutines in a closed loop and reports **req/s, p50, p99, and error rate**
(a response is an error only on transport failure or HTTP ≥ 500; `200`/`403` are
the normal mutant/human answers).

## k6 alternative

If you have k6 installed:

```sh
k6 run -e BASE_URL=http://127.0.0.1:8080 loadtest/mutant.js
```
