package service

import (
	"context"
	"fmt"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/google/uuid"
)

type SlidingWindowLimiter struct {
	store  *repository.RedisStore
	policy model.SlidingWindowPolicy
}

func NewSlidingWindowLimiter(policy model.SlidingWindowPolicy, store *repository.RedisStore) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		store:  store,
		policy: policy,
	}
}

func (r *SlidingWindowLimiter) RateLimit(
	ctx context.Context,
	clientID string,
	requestTime time.Time,
) (model.RateLimitingResponse, error) {
	if r.policy.Limit <= 0 {
		return model.RateLimitingResponse{}, fmt.Errorf("sliding window limit must be positive")
	}
	if r.policy.TimeWindow <= 0 {
		return model.RateLimitingResponse{}, fmt.Errorf("sliding window duration must be positive")
	}
	key := "rate-limit:window:" + clientID
	result, err := r.store.Eval(
		ctx,
		slidingWindowScript,
		[]string{key},
		requestTime.UnixNano(),
		r.policy.TimeWindow.Nanoseconds(),
		r.policy.Limit,
		uuid.NewString(),
	)
	if err != nil {
		return model.RateLimitingResponse{},
        fmt.Errorf("execute sliding window script: %w", err)
	}
	values,ok := result.([]interface{})
    if !ok || len(values) != 2 {
		return model.RateLimitingResponse{}, 
        fmt.Errorf("unexpected sliding window script result: %T", result)
	}

	allowedValue, ok := values[0].(int64)
    if !ok {
		return model.RateLimitingResponse{}, fmt.Errorf("unexpected allowed value type: %T", values[0])
	}

	retryAfter,ok := values[1].(int64)
    if !ok {
		return model.RateLimitingResponse{}, fmt.Errorf("unexpected retry-after value type: %T", values[1])
	}
    if allowedValue == 1 {
		return model.RateLimitingResponse{
			Allowed: true,
		}, nil
	}

	return model.RateLimitingResponse{
		Allowed:    false,
		RetryAfter: time.Duration(retryAfter).String(),
	},nil
}
