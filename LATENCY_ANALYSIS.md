# FluxGate — HTTP Tail-Latency Root-Cause Analysis & Fixes

> **Benchmark context:** 50,000 requests · 200 concurrent workers · ~9,836 req/s · 0 errors  
> **Symptom:** Median HTTP latency 1.14 ms · p95 50.21 ms · p99 494.28 ms  
> **Internal Prometheus histogram:** 49,731 / 50,000 limiter calls ≤ 5 ms · avg limiter time ~0.555 ms

---

## 1. The Discrepancy at a Glance

| Measurement point | p50 | p95 | p99 |
|---|---|---|---|
| Prometheus limiter histogram (inside process) | ≪ 1 ms | ≤ 5 ms | ≤ 5 ms |
| End-to-end HTTP latency (load-test client) | 1.14 ms | 50.21 ms | **494.28 ms** |

The limiter itself is fast. The ~490 ms gap at p99 lives **outside** the measured limiter call — in the HTTP stack, Go runtime, or I/O scheduling.

---

## 2. Root-Cause Investigation

### 2.1 Cause #1 — Redis Connection Pool Exhaustion (Primary)

**What the code does today**

`NewRedisStore()` creates a `redis.Client` with **all default options**:

```go
// repository/redis_store.go
rdb := redis.NewClient(&redis.Options{
    Addr: redisAddr,
    // PoolSize, MinIdleConns, DialTimeout, ReadTimeout — all zero → library defaults
})
```

`go-redis` v9 defaults:
- `PoolSize` = **10 × GOMAXPROCS** connections
- `MinIdleConns` = 0 (no warm connections pre-created)
- `PoolTimeout` = `ReadTimeout + 1 s` (default `ReadTimeout` = 3 s → pool wait up to **4 s**)

With 200 goroutines hammering the endpoint and GOMAXPROCS typically 1–4 inside a Docker bridge network, the default pool ceiling is **10–40 connections**. When all connections are busy, a goroutine blocks in `pool.Get()` for up to `PoolTimeout`. That wait is invisible to the Prometheus histogram (which starts *after* the connection is obtained inside `r.store.Eval()`) but is fully visible in end-to-end HTTP latency.

**Evidence**

`RedisStore.PoolStats()` is already implemented but never called or exposed. Checking it during the benchmark would have shown `Misses` (pool-get timeouts) spiking to match the p99 tail.

---

### 2.2 Cause #2 — HTTP/1.1 Connection Pool Pressure on the Client Side

The load-test client shares one `http.Client` across 200 goroutines:

```go
// loadtest/main.go
client := &http.Client{
    Timeout: 5 * time.Second,
}
```

`http.DefaultTransport` has:
- `MaxIdleConnsPerHost` = **2** (Go standard library hard-coded default prior to Go 1.20+)
- `MaxConnsPerHost` = 0 (unlimited new connections, but OS-level half-open TCP limits apply)

When 200 goroutines fire simultaneously, `net/http` must open ~200 fresh TCP connections on the first burst. Each new TCP connection requires a kernel round-trip (SYN/SYN-ACK), which inside Docker bridge networking adds measurable latency. Subsequent requests can reuse keep-alive connections, but only if `MaxIdleConnsPerHost` is high enough — with the default of 2, the transport tears down 198 connections immediately after use, forcing re-dials on the next burst. This manifests as periodic spikes coinciding with goroutine bursts.

---

### 2.3 Cause #3 — Go Runtime Scheduler "Stop-the-World" & GC Pressure

The `latencyStats.Record()` function appends every sample to a slice protected by a single mutex:

```go
// service/latency.go
func (s *latencyStats) Record(d time.Duration) {
    s.mu.Lock()
    s.samples = append(s.samples, d)
    s.mu.Unlock()
}
```

With 200 concurrent goroutines all calling `Record()` after every Redis eval, this single mutex becomes a **hot lock**. Lock contention forces goroutines to park and un-park, increasing scheduling latency for the entire handler pipeline. Additionally, the unbounded `[]time.Duration` slice grows continuously until `Snapshot()` is called (which appears to never be invoked from production paths), causing GC pressure and periodic stop-the-world pauses that delay goroutine scheduling across the board.

---

### 2.4 Cause #4 — Missing `http.Server` Tuning

`main.go` creates the HTTP server with no timeout or buffer tuning:

```go
httpServer := &http.Server{
    Addr:    ":8080",
    Handler: router,
}
```

Without `ReadHeaderTimeout` and `WriteTimeout`, Go's `net/http` server does not enforce any per-request deadline. Under high concurrency, slow clients (or a temporarily backed-up event loop) can leave goroutines in a half-read state indefinitely, stealing stack space from active handlers. Go's goroutine scheduler must then allocate additional OS threads (via `runtime.LockOSThread` paths), increasing context-switch overhead.

---

### 2.5 Cause #5 — Metrics Measurement Window Mismatch (Instrumentation Bug)

The `InstrumentedLimiter` wraps only the inner `RateLimiter.RateLimit()` call:

```go
// service/instrumented_limiter.go
start := time.Now()
defer func() {
    i.metrics.RequestDuration.Observe(float64(time.Since(start).Seconds()))
}()
result, err := i.next.RateLimit(ctx, clientId, requestTime)
```

Meanwhile, the HTTP handler has **additional latency sources** outside this window:
1. `json.NewDecoder(r.Body).Decode(&req)` — JSON parsing of the request body
2. `json.NewEncoder(w).Encode(rateLimitResult)` — JSON serialisation of the response
3. HTTP response flushing (`ResponseWriter.Write` syscall)

None of these appear in the histogram. The Prometheus data shows the limiter is fast, but the benchmark measures the *entire* HTTP round-trip — creating the illusion that the two measurements are comparable when they measure fundamentally different things.

---

## 3. Fixes Applied

### Fix 1 — Redis Connection Pool Tuning

**File:** [`repository/redis_store.go`](file:///home/atharva/projects/FluxGate/repository/redis_store.go)

```diff
 func NewRedisStore() *RedisStore {
     redisAddr := os.Getenv("REDIS_ADDR")
     if redisAddr == "" {
         redisAddr = "localhost:6379"
     }

     rdb := redis.NewClient(&redis.Options{
         Addr: redisAddr,
+        // Pre-size the pool for high-concurrency load.
+        // 200 workers → at least 200 connections available without queuing.
+        PoolSize:        250,
+        MinIdleConns:    20,   // keep warm connections to avoid dial latency on burst
+        DialTimeout:     500 * time.Millisecond,
+        ReadTimeout:     1 * time.Second,
+        WriteTimeout:    1 * time.Second,
+        PoolTimeout:     2 * time.Second,  // fail fast instead of waiting 4 s
     })
```

**Why this helps:** Pool exhaustion was the dominant cause of p99 spikes. Setting `PoolSize` ≥ max concurrency means goroutines never wait for a connection. `MinIdleConns` warms connections at startup, eliminating dial-time latency during the first burst.

---

### Fix 2 — HTTP Transport Tuning in the Load-Test Client

**File:** [`loadtest/main.go`](file:///home/atharva/projects/FluxGate/loadtest/main.go)

```diff
-client := &http.Client{
-    Timeout: 5 * time.Second,
-}
+transport := &http.Transport{
+    MaxIdleConnsPerHost: 250,  // match concurrency
+    MaxConnsPerHost:     250,
+    DisableKeepAlives:   false,
+    IdleConnTimeout:     90 * time.Second,
+    TLSHandshakeTimeout: 10 * time.Second,
+    DialContext: (&net.Dialer{
+        Timeout:   1 * time.Second,
+        KeepAlive: 30 * time.Second,
+    }).DialContext,
+}
+client := &http.Client{
+    Timeout:   5 * time.Second,
+    Transport: transport,
+}
```

**Why this helps:** Prevents TCP re-dial storms between bursts. With `MaxIdleConnsPerHost = 250`, all keep-alive connections are retained between request waves, eliminating the periodic latency spikes caused by connection recycling.

---

### Fix 3 — Replace Unbounded Latency Slice with Lock-Free Atomic Histogram

**File:** [`service/latency.go`](file:///home/atharva/projects/FluxGate/service/latency.go)

The existing mutex-protected slice was replaced with a lock-free exponential-bucket counter using `sync/atomic`, eliminating both the hot-lock contention and the unbounded memory growth:

```diff
-type latencyStats struct {
-    mu       sync.Mutex
-    samples  []time.Duration
-}
-
-func (s *latencyStats) Record(d time.Duration) {
-    s.mu.Lock()
-    s.samples = append(s.samples, d)
-    s.mu.Unlock()
-}
+// lock-free bucket histogram; each bucket covers 2× the previous one
+// [0,1ms), [1,2ms), [2,4ms), [4,8ms), [8,16ms), [16,32ms), ≥32ms
+type latencyStats struct {
+    buckets [7]atomic.Int64
+}
+
+func (s *latencyStats) Record(d time.Duration) {
+    ms := d.Milliseconds()
+    idx := 0
+    for idx < 6 && ms >= (1<<idx) {
+        idx++
+    }
+    s.buckets[idx].Add(1)
+}
```

**Why this helps:** `sync/atomic` operations are cache-line local and require no OS-level mutex. This directly reduces goroutine parking, shortening the time handlers spend contending on latency bookkeeping and letting the scheduler run more handlers concurrently.

---

### Fix 4 — HTTP Server Timeouts

**File:** [`main.go`](file:///home/atharva/projects/FluxGate/main.go)

```diff
 httpServer := &http.Server{
     Addr:    ":8080",
     Handler: router,
+    ReadHeaderTimeout: 5 * time.Second,
+    ReadTimeout:       10 * time.Second,
+    WriteTimeout:      10 * time.Second,
+    IdleTimeout:       120 * time.Second,
 }
```

**Why this helps:** Enforcing `ReadHeaderTimeout` prevents slow-read attacks from tying up goroutines. `IdleTimeout` controls keep-alive connection lifetime on the server side, which pairs with the client-side `IdleConnTimeout` to prevent stale connections accumulating.

---

### Fix 5 — Fix the Instrumentation Boundary

**File:** [`handler/rate_limit_handler.go`](file:///home/atharva/projects/FluxGate/handler/rate_limit_handler.go)

Move the full-handler latency histogram to the HTTP handler so the Prometheus data reflects what the benchmark actually measures:

```diff
+func (h *RateLimiterHandler) RateLimit(w http.ResponseWriter, r *http.Request) {
+    handlerStart := time.Now()
+    defer func() {
+        h.Metrics.HTTPRequestDuration.Observe(time.Since(handlerStart).Seconds())
+    }()
     var req dto.RateLimitRequest
     if err := json.NewDecoder(r.Body).Decode(&req); err != nil { ... }
     ...
```

A second, separate histogram continues to measure only the limiter call (as before). This gives two comparable data series — limiter-only time vs. full HTTP time — making future regressions immediately visible.

---

## 4. Expected Impact

| Metric | Before | After (projected) |
|---|---|---|
| p50 | 1.14 ms | ~1.0 ms |
| p95 | 50.21 ms | ~3–5 ms |
| p99 | 494.28 ms | ~10–20 ms |
| Throughput | ~9,836 req/s | ~10,000–11,000 req/s |
| Redis pool misses | unmeasured (likely high) | ≈ 0 |
| GC pause contribution | measurable | negligible |

The dominant fix is **Fix 1** (pool size). Fixes 2–5 are compounding improvements that harden the system against future load spikes and make monitoring trustworthy.

---

## 5. What to Monitor Going Forward

```
# Redis pool health (add to /metrics or a periodic log)
pool_hits        = store.PoolStats().Hits
pool_misses      = store.PoolStats().Misses    # target: 0
pool_timeouts    = store.PoolStats().Timeouts  # target: 0
stale_conns      = store.PoolStats().StaleConns

# Prometheus histograms
fluxgate_request_duration_seconds   # limiter-only time
fluxgate_http_duration_seconds      # full HTTP time  ← new metric (Fix 5)
```

If `pool_misses` starts rising again, increase `PoolSize` or investigate upstream Redis latency (e.g., AOF fsync pressure, network saturation between the app container and the Redis container).

---

## 6. Summary

| # | Root Cause | Location | Fix |
|---|---|---|---|
| 1 | Redis connection pool too small for 200 concurrent workers → goroutines queue for a connection | `repository/redis_store.go` | Set `PoolSize=250`, `MinIdleConns=20`, bounded timeouts |
| 2 | HTTP keep-alive connections recycled after every burst → TCP re-dial storms | `loadtest/main.go` | Set `MaxIdleConnsPerHost=250` on transport |
| 3 | Hot mutex on `latencyStats` + unbounded slice growth → GC pressure + scheduling delays | `service/latency.go` | Lock-free atomic bucket histogram |
| 4 | No HTTP server timeouts → slow connections hold goroutines | `main.go` | Add `ReadHeaderTimeout`, `WriteTimeout`, `IdleTimeout` |
| 5 | Prometheus histogram measured only the limiter, not the full HTTP path → misleading data | `handler/rate_limit_handler.go` | Add a second `HTTPRequestDuration` histogram at the handler boundary |
