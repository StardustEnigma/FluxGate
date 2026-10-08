package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
)

// TestTokenBucket_Concurrent_SharedClientDoesNotExceedCapacity fires N
// goroutines against a single shared clientID. Because the Lua script is
// atomic, the number of allowed responses must never exceed bucket capacity.
func TestTokenBucket_Concurrent_SharedClientDoesNotExceedCapacity(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const capacity = 10
	const goroutines = 50

	policy := model.TokenBucketPolicy{Capacity: capacity, RefillRate: 0.1}
	limiter := NewTokenBucketLimiter(policy, store)
	clientID := fmt.Sprintf("conc-tb-shared-%d", time.Now().UnixNano())
	requestTime := time.Now()

	var allowed, rejected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, clientID, requestTime)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	total := allowed.Load() + rejected.Load()
	if total != goroutines {
		t.Errorf("accounting mismatch: allowed=%d rejected=%d total=%d want=%d",
			allowed.Load(), rejected.Load(), total, goroutines)
	}
	if allowed.Load() > capacity {
		t.Errorf("allowed=%d exceeded capacity=%d — Lua atomicity violated",
			allowed.Load(), capacity)
	}
	t.Logf("goroutines=%d capacity=%d allowed=%d rejected=%d",
		goroutines, capacity, allowed.Load(), rejected.Load())
}

// TestTokenBucket_Concurrent_DistinctClients: first request per fresh clientID
// must always be allowed regardless of Redis concurrency.
func TestTokenBucket_Concurrent_DistinctClients(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const goroutines = 50
	policy := model.TokenBucketPolicy{Capacity: 5, RefillRate: 1}
	limiter := NewTokenBucketLimiter(policy, store)
	requestTime := time.Now()
	prefix := fmt.Sprintf("conc-tb-distinct-%d-", time.Now().UnixNano())

	var allowed, errCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		clientID := fmt.Sprintf("%s%d", prefix, i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, id, requestTime)
			if err != nil {
				errCount.Add(1)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			}
		}(clientID)
	}
	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("%d goroutines returned errors", errCount.Load())
	}
	if allowed.Load() != goroutines {
		t.Errorf("distinct-client first request should be allowed: got=%d want=%d",
			allowed.Load(), goroutines)
	}
	t.Logf("goroutines=%d allowed=%d errors=%d", goroutines, allowed.Load(), errCount.Load())
}

// TestTokenBucket_Concurrent_RefillUnderLoad drains the bucket, fast-forwards
// 1 second, and fires a concurrent burst; allowed must not exceed capacity.
func TestTokenBucket_Concurrent_RefillUnderLoad(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const capacity = 5
	policy := model.TokenBucketPolicy{Capacity: capacity, RefillRate: 5}
	limiter := NewTokenBucketLimiter(policy, store)
	clientID := fmt.Sprintf("conc-tb-refill-%d", time.Now().UnixNano())

	t0 := time.Now()
	for i := 0; i < capacity; i++ {
		resp, err := limiter.RateLimit(ctx, clientID, t0)
		if err != nil {
			t.Fatalf("drain phase error: %v", err)
		}
		if !resp.Allowed {
			t.Fatalf("drain phase: request %d should be allowed", i+1)
		}
	}

	t1 := t0.Add(time.Second)
	var allowed, rejected atomic.Int64
	var wg sync.WaitGroup
	const burst = 10
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, clientID, t1)
			if err != nil {
				t.Errorf("burst error: %v", err)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	if allowed.Load()+rejected.Load() != burst {
		t.Errorf("accounting mismatch: total=%d want=%d", allowed.Load()+rejected.Load(), burst)
	}
	if allowed.Load() > capacity {
		t.Errorf("allowed=%d exceeds capacity=%d after refill — atomicity violated",
			allowed.Load(), capacity)
	}
	t.Logf("burst=%d refill=1s allowed=%d rejected=%d", burst, allowed.Load(), rejected.Load())
}

// TestSlidingWindow_Concurrent_SharedClientDoesNotExceedLimit fires N
// goroutines against a single shared clientID.
func TestSlidingWindow_Concurrent_SharedClientDoesNotExceedLimit(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const limit = 10
	const goroutines = 50

	policy := model.SlidingWindowPolicy{Limit: limit, TimeWindow: 60 * time.Second}
	limiter := NewSlidingWindowLimiter(policy, store)
	clientID := fmt.Sprintf("conc-sw-shared-%d", time.Now().UnixNano())
	requestTime := time.Now()

	var allowed, rejected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, clientID, requestTime)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	total := allowed.Load() + rejected.Load()
	if total != goroutines {
		t.Errorf("accounting mismatch: allowed=%d rejected=%d total=%d want=%d",
			allowed.Load(), rejected.Load(), total, goroutines)
	}
	if allowed.Load() > limit {
		t.Errorf("allowed=%d exceeded window limit=%d — Lua atomicity violated",
			allowed.Load(), limit)
	}
	t.Logf("goroutines=%d limit=%d allowed=%d rejected=%d",
		goroutines, limit, allowed.Load(), rejected.Load())
}

// TestSlidingWindow_Concurrent_DistinctClients: distinct clients must not
// interfere with each other under concurrency.
func TestSlidingWindow_Concurrent_DistinctClients(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const goroutines = 50
	policy := model.SlidingWindowPolicy{Limit: 5, TimeWindow: 30 * time.Second}
	limiter := NewSlidingWindowLimiter(policy, store)
	requestTime := time.Now()
	prefix := fmt.Sprintf("conc-sw-distinct-%d-", time.Now().UnixNano())

	var allowed, errCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		clientID := fmt.Sprintf("%s%d", prefix, i)
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, id, requestTime)
			if err != nil {
				errCount.Add(1)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			}
		}(clientID)
	}
	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("%d goroutines returned errors", errCount.Load())
	}
	if allowed.Load() != goroutines {
		t.Errorf("distinct-client first request should be allowed: got=%d want=%d",
			allowed.Load(), goroutines)
	}
	t.Logf("goroutines=%d allowed=%d errors=%d", goroutines, allowed.Load(), errCount.Load())
}

// TestSlidingWindow_Concurrent_WindowEvictionUnderLoad fills a window, advances
// time past its boundary, then fires a concurrent burst.
func TestSlidingWindow_Concurrent_WindowEvictionUnderLoad(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	const limit = 5
	policy := model.SlidingWindowPolicy{Limit: limit, TimeWindow: 10 * time.Second}
	limiter := NewSlidingWindowLimiter(policy, store)
	clientID := fmt.Sprintf("conc-sw-eviction-%d", time.Now().UnixNano())

	t0 := time.Now()
	for i := 0; i < limit; i++ {
		resp, err := limiter.RateLimit(ctx, clientID, t0)
		if err != nil {
			t.Fatalf("fill phase error on %d: %v", i+1, err)
		}
		if !resp.Allowed {
			t.Fatalf("fill phase: request %d should be allowed", i+1)
		}
	}
	resp, err := limiter.RateLimit(ctx, clientID, t0)
	if err != nil {
		t.Fatalf("overflow check error: %v", err)
	}
	if resp.Allowed {
		t.Fatal("overflow request should have been rejected")
	}

	t1 := t0.Add(11 * time.Second)
	var allowed, rejected atomic.Int64
	var wg sync.WaitGroup
	const burst = 10
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := limiter.RateLimit(ctx, clientID, t1)
			if err != nil {
				t.Errorf("burst error: %v", err)
				return
			}
			if resp.Allowed {
				allowed.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	if allowed.Load()+rejected.Load() != burst {
		t.Errorf("accounting mismatch: total=%d want=%d", allowed.Load()+rejected.Load(), burst)
	}
	if allowed.Load() > int64(limit) {
		t.Errorf("allowed=%d exceeded limit=%d after eviction — atomicity violated",
			allowed.Load(), limit)
	}
	if allowed.Load() == 0 {
		t.Error("expected at least one allowed in fresh window")
	}
	t.Logf("burst=%d after expiry: allowed=%d rejected=%d", burst, allowed.Load(), rejected.Load())
}
