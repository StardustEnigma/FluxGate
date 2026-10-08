# FluxGate — Benchmark & Integration Test Verification Report

## 1. Test Suite Summary

All integration tests for both rate-limiting algorithms were executed against shared Redis state.

### Token Bucket Tests

Tests in `service/token_bucket_integration_test.go` and `service/concurrent_integration_test.go`:

- `TestTokenBucket_AllowRequests`: Baseline single-request permission.
- `TestTokenBucket_RejectsWhenEmpty`: Verifies capacity drain and rejection on empty.
- `TestTokenBucket_RefillsTokens`: Confirms continuous refill over time.
- `TestTokenBucket_DoesNotExceedsCapacity`: Ensures token accumulation is capped at `Capacity`.
- `TestTokenBucket_returnsRetryAfter`: Validates `Retry-After` estimation.
- `TestTokenBucket_Concurrent_SharedClientDoesNotExceedCapacity`: 50 concurrent goroutines targeting the same client ID with `capacity=10`. Allowed: 10, rejected: 40. Atomicity verified.
- `TestTokenBucket_Concurrent_DistinctClients`: 50 concurrent goroutines using isolated client IDs. All 50 permitted without cross-key contention or lock leaks.
- `TestTokenBucket_Concurrent_RefillUnderLoad`: Drains the bucket, simulates a 1-second time jump, and fires a 10-goroutine burst. Allowed requests matched the refilled tokens.

### Sliding Window Tests

Tests in `service/sliding_window_integeration_test.go` and `service/concurrent_integration_test.go`:

- `TestSlidingWindow_AllowRequests`: Baseline requests within the configured limit.
- `TestSlidingWindow_RejectsWhenLimitExceeds`: Verifies rejection after the request count reaches the limit.
- `TestSlidingWindow_AllowsAfterWindowExpiration`: Verifies timestamp eviction outside the window boundary.
- `TestSlidingWindow_ReturnsRetryAfter`: Validates retry duration relative to the oldest recorded timestamp.
- `TestSlidingWindow_Concurrent_SharedClientDoesNotExceedLimit`: 50 concurrent goroutines targeting one client ID with `limit=10`. Allowed: 10, rejected: 40. Atomicity verified.
- `TestSlidingWindow_Concurrent_DistinctClients`: 50 concurrent goroutines using distinct client IDs. All 50 permitted concurrently.
- `TestSlidingWindow_Concurrent_WindowEvictionUnderLoad`: Fills the window, advances past the window boundary, and verifies concurrent requests after eviction.

### Overall Test Result

```text
15 passed / 0 failed / 0 skipped
```

The integration suite completed successfully against shared Redis state.

---

## 2. Benchmark Environment

The controlled benchmark was executed against commit:

```text
e2447beeba53a39c8bb55ab3da25f80fa4caccd1
```

| Parameter | Specification |
|---|---|
| Host System | Windows 11 x86_64 |
| WSL2 Runtime | Ubuntu 24.04 LTS |
| Processor | AMD Ryzen 5 5600H, 6 cores / 12 threads |
| RAM | 7.5 GiB allocated to WSL2 |
| Physical RAM | 16 GiB |
| Go Toolchain | Go 1.24.6 linux/amd64 |
| Container Engine | Docker 29.2.1 |
| Redis | Redis 7 Alpine |
| Redis Networking | Docker bridge network |
| Redis Pool Size | 250 |
| Redis Minimum Idle Connections | 20 |
| Redis Dial Timeout | 500 ms |
| Redis Read Timeout | 1 s |
| Redis Write Timeout | 1 s |
| Redis Pool Timeout | 2 s |
| HTTP Server Read Header Timeout | 5 s |
| HTTP Server Read Timeout | 10 s |
| HTTP Server Write Timeout | 10 s |
| HTTP Server Idle Timeout | 120 s |
| HTTP Client Max Connections/Host | 250 |
| HTTP Keep-Alives | Enabled |

---

## 3. Controlled Benchmark Configuration

Both algorithms were tested using the same workload:

| Parameter | Value |
|---|---:|
| Total requests | 50,000 |
| Concurrency | 200 |
| Client IDs | 1,000 |
| Target | `http://localhost:8080/rate-limit` |
| Runs per algorithm | 3 |
| Errors permitted | 0 |

Client IDs were distributed using:

```text
client-%d
requestID % 1000
```

### Token Bucket

```text
Capacity:    10 tokens
Refill rate: 2 tokens/sec
```

### Sliding Window

```text
Limit:  10 requests
Window: 60 seconds
```

The benchmark runner starts Redis and FluxGate using Docker Compose, waits for both services to become ready, executes the load test, stores a self-describing JSON result, and cleans up the containers.

---

## 4. Final Benchmark Results

The following values are the arithmetic mean of three independent runs for each algorithm.

| Metric | Token Bucket | Sliding Window |
|---|---:|---:|
| **Throughput** | **22,537.74 req/s** | **23,894.00 req/s** |
| Average latency | 8.58 ms | 8.07 ms |
| P50 latency | 7.03 ms | 6.75 ms |
| P95 latency | 11.36 ms | 11.17 ms |
| P99 latency | 17.98 ms | 24.91 ms |
| Errors | 0 | 0 |

### Throughput

Sliding Window achieved approximately **6.0% higher average throughput** than Token Bucket under this workload.

```text
Token Bucket:    22,537.74 req/s
Sliding Window:  23,894.00 req/s
```

This is a workload-specific observation and should not be interpreted as evidence that Sliding Window is universally faster.

### Latency

Sliding Window produced slightly lower average, P50, and P95 latency:

```text
                 Token Bucket    Sliding Window
Average              8.58 ms          8.07 ms
P50                  7.03 ms          6.75 ms
P95                 11.36 ms         11.17 ms
```

Token Bucket showed a more stable P99 across the three runs. Sliding Window had one elevated P99 observation, which increased its three-run mean.

```text
Token Bucket P99:    17.98 ms
Sliding Window P99:  24.91 ms
```

---

## 5. Individual Benchmark Runs

### 5.1 Token Bucket

| Run | Throughput | Avg | P50 | P95 | P99 | Allowed | Rejected | Errors |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 22,996.82 req/s | 8.46 ms | 6.95 ms | 11.05 ms | 17.73 ms | 13,074 | 36,926 | 0 |
| 2 | 23,134.18 req/s | 8.39 ms | 6.93 ms | 10.93 ms | 18.64 ms | 13,133 | 36,867 | 0 |
| 3 | 21,482.23 req/s | 8.89 ms | 7.20 ms | 12.10 ms | 17.57 ms | 13,200 | 36,800 | 0 |
| **Mean** | **22,537.74 req/s** | **8.58 ms** | **7.03 ms** | **11.36 ms** | **17.98 ms** | — | — | **0** |

The number of allowed requests varies because tokens continue to refill during the benchmark.

---

### 5.2 Sliding Window

| Run | Throughput | Avg | P50 | P95 | P99 | Allowed | Rejected | Errors |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 24,045.23 req/s | 8.00 ms | 6.56 ms | 11.11 ms | 16.36 ms | 10,000 | 40,000 | 0 |
| 2 | 23,495.12 req/s | 8.25 ms | 6.91 ms | 11.46 ms | 41.02 ms | 10,000 | 40,000 | 0 |
| 3 | 24,141.66 req/s | 7.97 ms | 6.79 ms | 10.95 ms | 17.35 ms | 10,000 | 40,000 | 0 |
| **Mean** | **23,894.00 req/s** | **8.07 ms** | **6.75 ms** | **11.17 ms** | **24.91 ms** | — | — | **0** |

The Sliding Window implementation consistently allowed exactly 10,000 requests and rejected 40,000 under the configured policy.

---

## 6. Benchmark Interpretation

### Token Bucket

The Token Bucket implementation provides burst capacity and permits additional requests as tokens are replenished.

With:

```text
capacity   = 10
refill rate = 2 tokens/sec
```

approximately 13,000 requests were allowed during each benchmark run.

The implementation achieved:

- ~22.5k requests/sec average throughput
- 8.58 ms average latency
- 7.03 ms P50 latency
- 11.36 ms P95 latency
- 17.98 ms mean P99 latency
- 0 errors

### Sliding Window

The Sliding Window implementation enforces the configured request limit over a rolling time window.

With:

```text
limit  = 10 requests
window = 60 seconds
```

each run allowed exactly 10,000 requests across the 1,000 rotating client IDs.

The implementation achieved:

- ~23.9k requests/sec average throughput
- 8.07 ms average latency
- 6.75 ms P50 latency
- 11.17 ms P95 latency
- 24.91 ms mean P99 latency
- 0 errors

---

## 7. Prometheus Metrics Verification

The benchmark and integration-test work also verifies that request accounting is exposed through the application's Prometheus metrics.

The relevant metrics are:

```text
fluxgate_requests_total
fluxgate_allowed_total
fluxgate_rejected_total
fluxgate_errors_total
```

The client-side benchmark accounting and server-side Prometheus counters were previously verified to agree exactly for the benchmark workload:

```text
Total requests:  50,000
Allowed:         algorithm-dependent
Rejected:        algorithm-dependent
Errors:          0
```

The benchmark load test also validates that:

```text
allowed + rejected + errors = total requests
```

for every completed run.

---

## 8. Latency Measurement Boundaries

FluxGate exposes latency measurements at multiple architectural boundaries.

```text
[ Client: loadtest ]
        │
        │  End-to-End Latency
        │  HTTP client + network + server
        ▼
[ HTTP Handler ]
        │
        │  Full HTTP request duration
        │  JSON decode + limiter + JSON encode
        ▼
[ Instrumented Limiter ]
        │
        │  Limiter execution
        │  Redis acquisition + Lua execution + Redis round-trip
        ▼
[ Redis ]
```

These measurements answer different questions:

1. **Client end-to-end latency** measures what an external caller experiences.
2. **HTTP handler latency** measures application-level request processing.
3. **Limiter latency** measures the rate-limiting path, including Redis interaction.

The distinction is important when diagnosing tail latency. A difference between client and server measurements can include HTTP connection-pool behavior, network transit, scheduling, and response handling, while the limiter measurement isolates the rate-limiting path more closely.

---

## 9. Reproducing the Benchmark

### Token Bucket

Run:

```bash
ALGORITHM=token-bucket \
REQUESTS=50000 \
CONCURRENCY=200 \
./benchmark/run.sh
```

Optional algorithm configuration:

```bash
ALGORITHM=token-bucket \
CAPACITY=10 \
REFILL_RATE=2 \
REQUESTS=50000 \
CONCURRENCY=200 \
./benchmark/run.sh
```

### Sliding Window

Run:

```bash
ALGORITHM=sliding-window \
LIMIT=10 \
WINDOW=60s \
REQUESTS=50000 \
CONCURRENCY=200 \
./benchmark/run.sh
```

The runner records the Git commit and benchmark timestamp and generates a JSON result containing:

- algorithm
- commit
- timestamp
- target URL
- request count
- concurrency
- algorithm-specific configuration
- allowed requests
- rejected requests
- errors
- duration
- throughput
- average latency
- P50 latency
- P95 latency
- P99 latency

Algorithm-specific configuration is intentionally omitted when it does not apply. For example, a Token Bucket result contains `capacity` and `refill_rate`, while a Sliding Window result contains `limit` and `window`.

---

## 10. Benchmark Artifacts

The six final controlled runs are stored under:

```text
benchmark/results/
```

### Token Bucket

```text
benchmark-2026-10-08T18-03-02Z.json
benchmark-2026-10-08T18-03-14Z.json
benchmark-2026-10-08T18-03-26Z.json
```

### Sliding Window

```text
benchmark-2026-10-08T18-03-38Z.json
benchmark-2026-10-08T18-03-51Z.json
benchmark-2026-10-08T18-04-03Z.json
```

All six runs were executed against commit:

```text
e2447beeba53a39c8bb55ab3da25f80fa4caccd1
```

---

## 11. Conclusion

Under the controlled 50,000-request / 200-concurrency workload, both FluxGate algorithms completed successfully with **zero errors**.

Sliding Window achieved approximately **6.0% higher average throughput** and slightly lower average, P50, and P95 latency. Token Bucket exhibited a more stable P99 across the three runs, while Sliding Window showed one elevated P99 observation.

The results demonstrate the current implementation's behavior under a reproducible workload. They should not be interpreted as a universal performance ranking between rate-limiting algorithms, since performance depends on workload, configuration, hardware, Redis behavior, network conditions, and implementation details.

Combined with the **15 passing integration tests**, concurrency verification, Prometheus accounting, reproducible benchmark runner, and self-describing benchmark artifacts, this provides a repeatable baseline for future FluxGate performance comparisons.