#!/usr/bin/env bash
# Build + start the server on a throwaway DB, run the loadgen driver (reads then
# mixed), and tear everything down. Invoked by `make loadtest`.
#
# Env overrides: PORT, DURATION, LEVELS.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${PORT:-8300}"
DURATION="${DURATION:-5s}"
LEVELS="${LEVELS:-1,8,32,64,128}"

TMP="$(mktemp -d)"
SRV=""
cleanup() {
  status=$?
  if [ -n "$SRV" ]; then
    kill "$SRV" 2>/dev/null || true
    wait "$SRV" 2>/dev/null || true
  fi
  # On failure, surface the server log before deleting it — otherwise startup
  # errors (e.g. port already in use) are undiagnosable.
  if [ "$status" -ne 0 ] && [ -s "$TMP/server.log" ]; then
    echo "--- server.log (exit $status) ---" >&2
    cat "$TMP/server.log" >&2
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

mkdir -p "$TMP/fe"
printf '<h1>mutant detector</h1>' > "$TMP/fe/index.html"

echo "building server + loadgen..."
( cd "$ROOT/backend" && go build -o "$TMP/server" ./cmd/server )
( cd "$ROOT/loadtest/loadgen" && go build -o "$TMP/loadgen" . )

PORT="$PORT" DB_PATH="$TMP/m.db" FRONTEND_DIR="$TMP/fe" "$TMP/server" >"$TMP/server.log" 2>&1 &
SRV=$!

# Fail fast if the server died on startup, rather than letting loadgen burn
# its full 10s health-check timeout before reporting an opaque error.
sleep 0.2
if ! kill -0 "$SRV" 2>/dev/null; then
  echo "server exited immediately after launch" >&2
  SRV=""
  exit 1
fi

echo
echo "== reads (GET /stats/, O(1) counters) =="
"$TMP/loadgen" -url "http://127.0.0.1:$PORT" -levels "$LEVELS" -duration "$DURATION" -mix reads

echo
echo "== mixed (90% write / 10% stats) =="
"$TMP/loadgen" -url "http://127.0.0.1:$PORT" -levels "$LEVELS" -duration "$DURATION" -mix mixed
