# FluxGate — Benchmark & Integration Test Verification Report

## 1. Test Suite Summary

All integration tests for both rate-limiting algorithms were executed against shared Redis state:

- **Token Bucket Tests** (in [`service/token_bucket_integration_test.go`](file:///home/atharva/projects/FluxGate/service/token_bucket_integration_test.go) and [`service/concurrent_integration_test.go`](file:///home/atharva/projects/FluxGate/service/concurrent_integration_test.go)):
  - `TestTokenBucket_AllowRequests`: Baseline single-request permission.
  - `TestTokenBucket_RejectsWhenEmpty`: Verifies capacity drain and rejection on empty.
  - `TestTokenBucket_RefillsTokens`: Confirms continuous refill over time.
  - `TestTokenBucket_DoesNotExceedsCapacity`: Ensures bucket token accumulation caps at `Capacity`.
  - `TestTokenBucket_returnsRetryAfter`: Validates standard `Retry-After` estimation.
  - `TestTokenBucket_Concurrent_SharedClientDoesNotExceedCapacity`: **50 concurrent goroutines** targeting the **same shared client ID** (`capacity=10`). Allowed: `10`, Rejected: `40`. Atomicity verified.
  - `TestTokenBucket_Concurrent_DistinctClients`: **50 concurrent goroutines** each with an isolated client ID. All 50 permitted without cross-key contention or lock leaks.
  - `TestTokenBucket_Concurrent_RefillUnderLoad`: Drains bucket, simulates 1 s time jump, and fires a 10-goroutine concurrent burst; allowed requests exactly matched refilled tokens (`5`).

- **Sliding Window Tests** (in [`service/sliding_window_integeration_test.go`](file:///home/atharva/projects/FluxGate/service/sliding_window_integeration_test.go) and [`service/concurrent_integration_test.go`](file:///home/atharva/projects/FluxGate/service/concurrent_integration_test.go)):
  - `TestSlidingWindow_AllowRequests`: Baseline window requests within limit.
  - `TestSlidingWindow_RejectsWhenLimitExceeds`: Rejection after count equals limit.
  - `TestSlidingWindow_AllowsAfterWindowExpiration`: Old timestamps evicted outside window boundary.
  - `TestSlidingWindow_ReturnsRetryAfter`: Validates retry duration relative to oldest recorded timestamp.
  - `TestSlidingWindow_Concurrent_SharedClientDoesNotExceedLimit`: **50 concurrent goroutines** against a **single client ID** (`limit=10`). Allowed: `10`, Rejected: `40`. Atomicity verified.
  - `TestSlidingWindow_Concurrent_DistinctClients`: **50 concurrent goroutines** with distinct client IDs; all 50 allowed concurrently.
  - `TestSlidingWindow_Concurrent_WindowEvictionUnderLoad`: Fills window, advances 11 s past window boundary, fires 10 concurrent requests; exactly `5` accepted up to the limit.

**Overall Test Result:** `15 passed / 0 failed / 0 skipped (0.128s)`

---

## 2. Benchmark Environment & Configuration

| Parameter | Specification |
|---|---|
| **Host System** | Windows 11 x86_64 |
| **WSL2 Runtime** | Ubuntu 24.04 LTS (`Linux 5.15.167.4-microsoft-standard-WSL2`) |
| **Processor** | AMD Ryzen 5 5600H with Radeon Graphics (6 Cores / 12 Threads @ up to 4.2 GHz) |
| **RAM** | 7.5 GiB allocated to WSL2 VM (16 GiB physical system RAM) |
| **Go Toolchain** | go1.24.6 linux/amd64 |
| **Container Engine** | Docker 29.2.1 |
| **Redis Server** | Redis 7.4.9 (`redis:7-alpine`), jemalloc 5.3.0, ephemeral single-node |
| **Redis Networking** | Docker bridge network (`fluxgate-network`), port 6379 |
| **Go Redis Pool (`RedisStore`)** | `PoolSize`: 250, `MinIdleConns`: 20, `DialTimeout`: 500ms, `ReadTimeout`: 1s, `WriteTimeout`: 1s, `PoolTimeout`: 2s |
| **HTTP Server (`http.Server`)** | `ReadHeaderTimeout`: 5s, `ReadTimeout`: 10s, `WriteTimeout`: 10s, `IdleTimeout`: 120s |
| **HTTP Client Transport** | `MaxIdleConnsPerHost`: 250, `MaxConnsPerHost`: 250, Keep-Alives enabled, 90s idle timeout |

---

## 3. Benchmark Execution Results

### 3.1 Token Bucket Algorithm
- **Configuration**: Capacity = 10 tokens, Refill Rate = 2 tokens/sec
- **Client-ID Distribution**: 1,000 distinct client IDs (`client-0` through `client-999`, distributed via `requestID % 1000`)
- **Total Requests**: 50,000
- **Concurrency**: 200 worker goroutines

| Metric | Load-Test Client Measurement |
|---|---|
| **Allowed Requests** | 14,000 |
| **Rejected Requests** | 36,000 |
| **Errors** | 0 |
| **Accounted** | 50,000 (100% accounting pass) |
| **Total Duration** | 2.483 s |
| **Throughput** | **20,134.49 req/s** |
| **Average Latency** | 9.60 ms |
| **p50 Latency** | 8.36 ms |
| **p95 Latency** | **15.48 ms** |
| **p99 Latency** | **26.60 ms** |

---

### 3.2 Sliding Window Algorithm
- **Configuration**: Limit = 10 requests, Time Window = 60s
- **Client-ID Distribution**: 1,000 distinct client IDs (`client-0` through `client-999`, distributed via `requestID % 1000`)
- **Total Requests**: 50,000
- **Concurrency**: 200 worker goroutines

| Metric | Load-Test Client Measurement |
|---|---|
| **Allowed Requests** | 10,000 (1,000 clients × 10 quota limit) |
| **Rejected Requests** | 40,000 (1,000 clients × 40 overflow requests) |
| **Errors** | 0 |
| **Accounted** | 50,000 (100% accounting pass) |
| **Total Duration** | 2.276 s |
| **Throughput** | **21,971.05 req/s** |
| **Average Latency** | 8.84 ms |
| **p50 Latency** | 7.64 ms |
| **p95 Latency** | **13.15 ms** |
| **p99 Latency** | **20.30 ms** |

---

## 4. Prometheus Metrics Verification & Measurement Boundary Reconcile

### 4.1 Request Counts Agreement

| Measurement | Token Bucket (Client) | Token Bucket (Prometheus) | Sliding Window (Client) | Sliding Window (Prometheus) | Status |
|---|---|---|---|---|---|
| **Total Requests** | 50,000 | `fluxgate_requests_total` = 50,000 | 50,000 | `fluxgate_requests_total` = 50,000 | **Exact Match (100%)** |
| **Allowed Requests** | 14,000 | `fluxgate_allowed_total` = 14,000 | 10,000 | `fluxgate_allowed_total` = 10,000 | **Exact Match (100%)** |
| **Rejected Requests** | 36,000 | `fluxgate_rejected_total` = 36,000 | 40,000 | `fluxgate_rejected_total` = 40,000 | **Exact Match (100%)** |
| **Errors** | 0 | `fluxgate_errors_total` = 0 | 0 | `fluxgate_errors_total` = 0 | **Exact Match (100%)** |

### 4.2 Measurement Boundary Hierarchy & Latency Analysis

There are three distinct observation boundaries in the architecture:

```
[ Client: loadtest ]
      │  (1) End-to-End Latency: captures client runtime, TCP handshake/pool checkout, OS socket, RTT, JSON decode
      ▼
[ Server: handler.RateLimiterHandler ]
      │  (2) HTTP Handler Histogram (fluxgate_http_request_duration_seconds):
      │      JSON decode + limiter decision + JSON encode
      ▼
[ Service: service.InstrumentedLimiter ]
         (3) Limiter Histogram (fluxgate_request_duration_seconds):
             Redis connection acquisition, Lua Eval execution, Redis network round-trip
```

#### Latency Quantile Comparison (Token Bucket, 50k reqs @ 200 concurrency):

| Measurement Point | Average | p50 | p95 | p99 |
|---|---|---|---|---|
| **Client End-to-End** (`loadtest`) | 9.60 ms | 8.36 ms | 15.48 ms | 26.60 ms |
| **Server Full HTTP** (`fluxgate_http_request_duration_seconds`) | 7.00 ms | ~7.05 ms | ~18.23 ms | ~24.01 ms |
| **Limiter Internal / Redis** (`fluxgate_request_duration_seconds`) | 6.98 ms | ~7.03 ms | ~18.20 ms | ~23.95 ms |

#### Explanation of Boundary Differences:
1. **Client End-to-End vs. Server HTTP Duration (Delta ~2.60 ms average, ~1.31 ms p50):**
   - The client measures round-trip time starting immediately before invoking `client.Do(req)` until HTTP headers are decoded.
   - The delta captures:
     - Client-side `http.Transport` connection acquisition from pool
     - Kernel network stack transit across the Docker bridge / loopback virtual interface
     - Server-side goroutine dispatch inside `http.Server` before reaching `RateLimiterHandler.RateLimit`
     - Response header and buffer flush over the socket back to the client.
2. **Server HTTP Duration vs. Limiter-Only Duration (Delta ~0.02 ms / 20 microseconds):**
   - Captures JSON request body decoding (`json.NewDecoder(r.Body).Decode(&req)`) and JSON response serialization (`json.NewEncoder(w).Encode(...)`).
   - Confirms that serialization overhead in Go is negligible (< 0.3% of server processing time).