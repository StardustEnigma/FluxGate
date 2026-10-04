package service

import (
	"context"
	"fmt"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
)

type TokenBucketLimiter struct {
	store  *repository.RedisStore
	policy model.TokenBucketPolicy
}

func NewTokenBucketLimiter(policy model.TokenBucketPolicy, store *repository.RedisStore) *TokenBucketLimiter {

	return &TokenBucketLimiter{
		store:  store,
		policy: policy,
	}
}

func (r *TokenBucketLimiter) RateLimit(ctx context.Context, Clientid string, requestTime time.Time) (model.RateLimitingResponse, error) {
	if r.policy.RefillRate <= 0 {
		return model.RateLimitingResponse{},
			fmt.Errorf("token bucket refill rate must be positive")
	}

	if r.policy.Capacity < 1 {
		return model.RateLimitingResponse{},
			fmt.Errorf("token bucket capacity must be at least 1")
	}
	key := "rate-limit:bucket:" + Clientid
	result, err := r.store.Eval(
		ctx,
		tokenBucketScript,
		[]string{key},
		r.policy.Capacity,
		r.policy.RefillRate,
		requestTime.UnixNano(),
	)
	if err != nil {
		return model.RateLimitingResponse{},
			fmt.Errorf("execute token bucket script: %w", err)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 3 {
		return model.RateLimitingResponse{},
			fmt.Errorf("invalid token bucket result: expected 3 values")
	}

	allowed, ok := values[0].(int64)
	if !ok {
		return model.RateLimitingResponse{},
			fmt.Errorf("invalid allowed value returned by token bucket script")
	}
	_, ok = values[1].(int64)
	if !ok {
		return model.RateLimitingResponse{},
			fmt.Errorf("invalid current token count returned by token bucket script")
	}
	retryAfter, ok := values[2].(int64)
	if !ok {
		return model.RateLimitingResponse{},
			fmt.Errorf("invalid retry-after value returned by token bucket script")
	}
	response := model.RateLimitingResponse{
		Allowed: allowed == 1,
	}
	if !response.Allowed {
		duration := time.Duration(retryAfter) * time.Millisecond
		response.RetryAfter = duration.String()
	}

	return response,nil
}
