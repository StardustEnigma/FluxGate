#!/usr/bin/env bash

set -euo pipefail

# --------------------------------------------------
# FluxGate Reproducible Benchmark
# --------------------------------------------------

# Algorithm configuration
ALGORITHM="${ALGORITHM:-token-bucket}"

# Token Bucket configuration
CAPACITY="${CAPACITY:-10}"
REFILL_RATE="${REFILL_RATE:-2}"

# Sliding Window configuration
LIMIT="${LIMIT:-10}"
WINDOW="${WINDOW:-60s}"

# Validate algorithm
case "$ALGORITHM" in
    token-bucket)
        ;;
    sliding-window)
        ;;
    *)
        echo "Unsupported algorithm: $ALGORITHM"
        echo "Use: token-bucket or sliding-window"
        exit 1
        ;;
esac

# --------------------------------------------------
# Benchmark configuration
# --------------------------------------------------

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RESULT_DIR="$ROOT_DIR/benchmark/results"

REQUESTS="${REQUESTS:-50000}"
CONCURRENCY="${CONCURRENCY:-200}"
PORT="${PORT:-8080}"

TIMESTAMP="$(date -u +"%Y-%m-%dT%H-%M-%SZ")"
COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"

mkdir -p "$RESULT_DIR"

# --------------------------------------------------
# Print benchmark configuration
# --------------------------------------------------

echo "=========================================="
echo "       FluxGate Reproducible Benchmark"
echo "=========================================="
echo "Commit:       $COMMIT"
echo "Timestamp:    $TIMESTAMP"
echo "Algorithm:    $ALGORITHM"

if [[ "$ALGORITHM" == "token-bucket" ]]; then
    echo "Capacity:     $CAPACITY"
    echo "Refill rate:  $REFILL_RATE"
else
    echo "Limit:        $LIMIT"
    echo "Window:       $WINDOW"
fi

echo "Requests:     $REQUESTS"
echo "Concurrency:  $CONCURRENCY"
echo

cd "$ROOT_DIR"

# --------------------------------------------------
# 1. Start Redis
# --------------------------------------------------

echo "[1/5] Starting infrastructure..."

docker compose up -d redis

# --------------------------------------------------
# 2. Wait for Redis
# --------------------------------------------------

echo "[2/5] Waiting for Redis..."

until docker compose exec -T redis redis-cli ping | grep -q PONG; do
    sleep 1
done

echo "Redis is ready."

# --------------------------------------------------
# 3. Start FluxGate
# --------------------------------------------------

echo "[3/5] Starting FluxGate..."

ALGORITHM="$ALGORITHM" \
CAPACITY="$CAPACITY" \
REFILL_RATE="$REFILL_RATE" \
LIMIT="$LIMIT" \
WINDOW="$WINDOW" \
docker compose up -d fluxgate

# --------------------------------------------------
# 4. Wait for FluxGate
# --------------------------------------------------

echo "[4/5] Waiting for FluxGate..."

until curl -sf "http://localhost:${PORT}/metrics" > /dev/null; do
    sleep 1
done

echo "FluxGate is ready."

# --------------------------------------------------
# 5. Run benchmark
# --------------------------------------------------

echo "[5/5] Running benchmark..."

RESULT_FILE="$RESULT_DIR/benchmark-${TIMESTAMP}.json"

go run ./loadtest \
    --url "http://localhost:${PORT}/rate-limit" \
    --requests "$REQUESTS" \
    --concurrency "$CONCURRENCY" \
    --algorithm "$ALGORITHM" \
    --capacity "$CAPACITY" \
    --refill-rate "$REFILL_RATE" \
    --limit "$LIMIT" \
    --window "$WINDOW" \
    --commit "$COMMIT" \
    --timestamp "$TIMESTAMP" \
    --json > "$RESULT_FILE"
# --------------------------------------------------
# Results
# --------------------------------------------------

echo
echo "=========================================="
echo "Benchmark complete"
echo "=========================================="
echo "Result: $RESULT_FILE"
echo

cat "$RESULT_FILE"

# --------------------------------------------------
# Cleanup
# --------------------------------------------------

echo
echo "Cleaning up..."

docker compose down

echo "Done."