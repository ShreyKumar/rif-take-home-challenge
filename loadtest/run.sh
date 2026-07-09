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
  [ -n "$SRV" ] && kill "$SRV" 2>/dev/null || true
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

echo
echo "== reads (GET /stats/, O(1) counters) =="
"$TMP/loadgen" -url "http://127.0.0.1:$PORT" -levels "$LEVELS" -duration "$DURATION" -mix reads

echo
echo "== mixed (90% write / 10% stats) =="
"$TMP/loadgen" -url "http://127.0.0.1:$PORT" -levels "$LEVELS" -duration "$DURATION" -mix mixed
