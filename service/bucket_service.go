package service

import (
	"context"
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

func (r *TokenBucketLimiter) RateLimit(ctx context.Context, Clientid string, requestTime time.Time) model.RateLimitingResponse {

	key := "rate-limit:bucket:" + Clientid
	result ,err := r.store.Eval(
		ctx,
		tokenBucketScript,
		[]string{key},
		r.policy.Capacity,
		r.policy.RefillRate,
		requestTime.UnixNano(),
	)
	if err != nil {
		return model.RateLimitingResponse{}
	}
	
	values,ok := result.([]interface{})
	if !ok || len(values) != 3 {
		return model.RateLimitingResponse{}
	}

	allowed,ok := values[0].(int64)
	if !ok {
		return model.RateLimitingResponse{}
	}
	retryAfter,ok := values[2].(int64)
	if !ok {
		return model.RateLimitingResponse{}
	}
	response :=model.RateLimitingResponse{
		Allowed: allowed==1,
	}
	if !response.Allowed {
		duration := time.Duration(retryAfter) * time.Millisecond
		response.RetryAfter=duration.String() 
	}
	
	return response
}