package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/redis/go-redis/v9"
)

type TokenBucketLimiter struct {
	store  *repository.RedisStore
	policy model.TokenBucketPolicy
}

func NewTokenBucketLimiter(policy model.TokenBucketPolicy, store *repository.RedisStore) *TokenBucketLimiter {
	fmt.Printf(
		"Token Bucket Policy: capacity=%f refillRate=%f\n",
		policy.Capacity,
		policy.RefillRate,
	)
	return &TokenBucketLimiter{
		store:  store,
		policy: policy,
	}
}

func (r *TokenBucketLimiter) RateLimit(ctx context.Context, Clientid string, requestTime time.Time) model.RateLimitingResponse {

	key := "rate-limit:bucket:" + Clientid

	data, err := r.store.HGetAll(ctx, key)
	fmt.Printf("REDIS DATA: %+v\n", data)
fmt.Printf("HGET ERROR: %v\n", err)
	var bucket model.Bucket

	if err == nil {
		bucket, err := bucketFromHash(data)
		if err != nil {
			return model.RateLimitingResponse{}
		}

		timeDiff := requestTime.Sub(bucket.LastRefill)
		newTokens := timeDiff.Seconds() * (bucket.RefillRate)

		if newTokens > 0 {
			bucket.CurrentTokens = min(bucket.CurrentTokens+newTokens, bucket.Capacity)
			bucket.LastRefill = requestTime
		}

	} else if errors.Is(err, redis.Nil) {
		bucket.Capacity = r.policy.Capacity
		bucket.CurrentTokens = r.policy.Capacity
		bucket.RefillRate = r.policy.RefillRate
		bucket.LastRefill = requestTime
	} else {
		return model.RateLimitingResponse{}
	}
	var results model.RateLimitingResponse
	if bucket.CurrentTokens >= 1.0 {
		bucket.CurrentTokens -= 1.0
		results.Allowed = true
	} else {
		fmt.Printf(
			"tokens=%f refillRate=%f capacity=%f lastRefill=%v requestTime=%v\n",
			bucket.CurrentTokens,
			bucket.RefillRate,
			bucket.Capacity,
			bucket.LastRefill,
			requestTime,
		)
		results.Allowed = false
		seconds := (1 - bucket.CurrentTokens) / bucket.RefillRate
		retryAfter := time.Duration(
			seconds * float64(time.Second),
		)
		results.RetryAfter = retryAfter.String()
	}

	if err := r.store.HSet(
		ctx,
		key,
		"capacity", bucket.Capacity,
		"refillRate", bucket.RefillRate,
		"currentTokens", bucket.CurrentTokens,
		"lastRefill", bucket.LastRefill.UnixNano(),
	); err != nil {
		return model.RateLimitingResponse{}
	}
	return results
}
func bucketFromHash(data map[string]string) (model.Bucket, error) {
	var bucket model.Bucket

	capacity, err := strconv.ParseFloat(data["capacity"], 64)
	if err != nil {
		return bucket, err
	}
	refillRate, err := strconv.ParseFloat(data["refillRate"], 64)
	if err != nil {
		return bucket, err
	}
	currentTokens, err := strconv.ParseFloat(data["currentTokens"], 64)
	if err != nil {
		return bucket, err
	}
	lastRefill, err := strconv.ParseInt(data["lastRefill"], 10, 64)
	if err != nil {
		return bucket, err
	}
	bucket.Capacity = capacity
	bucket.RefillRate = refillRate
	bucket.CurrentTokens = currentTokens
	bucket.LastRefill = time.Unix(0, lastRefill)
	return bucket, nil
}
