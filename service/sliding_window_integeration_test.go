package service

import (
	"context"
	"testing"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
)

func TestSlidingWindow_AllowRequests(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("redis is not running")
	}

	policy := model.SlidingWindowPolicy{
		Limit:      3,
		TimeWindow: 10 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(policy, store)
	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	for i := 0; i < 3; i++ {
		response := callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime,
		)
		if !response.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
}
func TestSlidingWindow_RejectsWhenLimitExceeds(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("redis is not running")
	}
	policy := model.SlidingWindowPolicy{
		Limit:      3,
		TimeWindow: 10 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(policy, store)
	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")
	requestTime := time.Now()
	for i := 0; i < 3; i++ {
		response := callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime,
		)
		if !response.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	response := callSlidingWindow(
		t,
		limiter,
		ctx,
		clientId,
		requestTime,
	)
	if response.Allowed {
		t.Fatalf("request should have been rejected due to exceeded limit")
	}
}

func TestSlidingWindow_AllowsAfterWindowExpiration(t *testing.T) {
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("reddis is not running")
	}
	policy := model.SlidingWindowPolicy{
		Limit:      3,
		TimeWindow: 10 * time.Second,
	}
	limiter := NewSlidingWindowLimiter(policy, store)
	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")
	requestTime := time.Now()

	for i := 0; i < 3; i++ {
		response := callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime,
		)

		if !response.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	
	response := callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime.Add(5*time.Second),
		)

	if response.Allowed {
		t.Fatalf("request should have been rejected")
	}

	response = callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime.Add(10*time.Second),
		)

	if !response.Allowed {
		t.Fatalf("request should have been allowed")
	}
}
func TestSlidingWindow_ReturnsRetryAfter(t *testing.T) {

	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Skip("Redis is not running")
	}

	policy := model.SlidingWindowPolicy{
		Limit:      1,
		TimeWindow: 10 * time.Second,
	}

	limiter := NewSlidingWindowLimiter(policy, store)

	clientId := "test-client-" + time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	response := callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime,
		)

	if !response.Allowed {
		t.Fatal("first request should have been allowed")
	}

	response = callSlidingWindow(
			t,
			limiter,
			ctx,
			clientId,
			requestTime,
		)

	if response.Allowed {
		t.Fatal("second request should have been rejected")
	}

	if response.RetryAfter == "" {
		t.Error("expected RetryAfter for rejected request")
	}
}
func callSlidingWindow(
	t *testing.T,
	limiter *SlidingWindowLimiter,
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
