package service

import (
	"context"
	"testing"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
)

func TestTokenBucket_AllowRequests(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}
	policy := model.TokenBucketPolicy{
		Capacity:   5,
		RefillRate: 1,
	}
	limiter := NewTokenBucketLimiter(policy, store)

	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	response := callTokenBucket(t, limiter, ctx, clientId, time.Now())

	if !response.Allowed {
		t.Error("expected request to be allowed")
	}
}

func TestTokenBucket_RejectsWhenEmpty(t *testing.T) {

	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	policy := model.TokenBucketPolicy{
		Capacity:   3,
		RefillRate: 1,
	}

	limiter := NewTokenBucketLimiter(policy, store)

	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	for i := 0; i < 3; i++ {

		response := callTokenBucket(t, limiter, ctx, clientId, requestTime)

		if !response.Allowed {
			t.Fatalf("request %d should have been Allowed", i+1)
		}
	}
	response := callTokenBucket(t, limiter, ctx, clientId, requestTime)
	if response.Allowed {
		t.Error("expected request to be rejected when bucket is empty")
	}
}

func TestTokenBucket_RefillsTokens(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not runnig")
	}
	policy := model.TokenBucketPolicy{
		Capacity:   1,
		RefillRate: 1,
	}

	limiter := NewTokenBucketLimiter(policy, store)
	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	response := callTokenBucket(t, limiter, ctx, clientId, requestTime)

	if !response.Allowed {
		t.Fatalf("first request should have been allowed")
	}

	response = callTokenBucket(t, limiter, ctx, clientId, requestTime)
	if response.Allowed {
		t.Fatalf("second request should have been rejected")
	}
	refilledTime := requestTime.Add(time.Second)
	response = callTokenBucket(t, limiter, ctx, clientId, refilledTime)

	if !response.Allowed {
		t.Error("request should have been allowed after token refill")
	}
}
func TestTokenBucket_DoesNotExceedsCapacity(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}
	policy := model.TokenBucketPolicy{
		Capacity:   3,
		RefillRate: 10,
	}
	limiter := NewTokenBucketLimiter(policy, store)

	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")
	requestTime := time.Now()

	for i := 0; i < 3; i++ {
		response := callTokenBucket(t, limiter, ctx, clientId, requestTime)
		if !response.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	refilledTime := requestTime.Add(10 * time.Second)

	for i := 0; i < 3; i++ {
		response := callTokenBucket(t, limiter, ctx, clientId, refilledTime)
		if !response.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	response := callTokenBucket(t, limiter, ctx, clientId, refilledTime)

	if response.Allowed {
		t.Fatalf("bucket exceeded its configuration capacity")
	}
}

func TestTokenBucket_returnsRetryAfter(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}
	policy := model.TokenBucketPolicy{
		Capacity:   1,
		RefillRate: 1,
	}
	limiter := NewTokenBucketLimiter(policy, store)
	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	response := callTokenBucket(t, limiter, ctx, clientId, requestTime)

	if !response.Allowed {
		t.Fatalf("first request should have been allowed")
	}

	response = callTokenBucket(t, limiter, ctx, clientId, requestTime)
	if response.Allowed {
		t.Fatalf("second request should have been rejected")
	}

	if response.RetryAfter == "" {
		t.Error("expected retry after for the rejected request")
	}

}

func callTokenBucket(
	t *testing.T,
	limiter *TokenBucketLimiter,
	ctx context.Context,
	clientID string,
	requestTime time.Time,
) model.RateLimitingResponse {
	t.Helper()

	response, err := limiter.RateLimit(ctx, clientID, requestTime)
	if err != nil {
		t.Fatalf("RateLimit() returned unexpected error: %v", err)
	}

	return response
}
