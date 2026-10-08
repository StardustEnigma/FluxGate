#!/usr/bin/env bash

set -euo pipefail

# --------------------------------------------------
# FluxGate Reproducible Benchmark
# --------------------------------------------------

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESULT_DIR="$ROOT_DIR/benchmark/results"

REQUESTS="${REQUESTS:-50000}"
CONCURRENCY="${CONCURRENCY:-200}"
PORT="${PORT:-8080}"

TIMESTAMP="$(date -u +"%Y-%m-%dT%H-%M-%SZ")"
COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"

mkdir -p "$RESULT_DIR"

echo "=========================================="
echo "       FluxGate Reproducible Benchmark"
echo "=========================================="
echo "Commit:       $COMMIT"
echo "Timestamp:    $TIMESTAMP"
echo "Requests:     $REQUESTS"
echo "Concurrency:  $CONCURRENCY"
echo

cd "$ROOT_DIR"

echo "[1/5] Starting infrastructure..."
docker compose up -d redis

echo "[2/5] Waiting for Redis..."

until docker compose exec -T redis redis-cli ping | grep -q PONG; do
    sleep 1
done

echo "Redis is ready."

echo "[3/5] Starting FluxGate..."

docker compose up -d fluxgate

echo "[4/5] Waiting for FluxGate..."

until curl -sf "http://localhost:${PORT}/metrics" > /dev/null; do
    sleep 1
done

echo "FluxGate is ready."

echo "[5/5] Running benchmark..."

RESULT_FILE="$RESULT_DIR/benchmark-${TIMESTAMP}.json"

go run ./loadtest \
    --url "http://localhost:${PORT}/rate-limit" \
    --requests "$REQUESTS" \
    --concurrency "$CONCURRENCY" \
    --json > "$RESULT_FILE"

echo
echo "=========================================="
echo "Benchmark complete"
echo "=========================================="
echo "Result: $RESULT_FILE"
echo

cat "$RESULT_FILE"

echo
echo "Cleaning up..."

docker compose down

echo "Done."