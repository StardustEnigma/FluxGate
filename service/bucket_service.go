package service

import (
	"context"
	"errors"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/redis/go-redis/v9"
)

type TokenBucketLimiter struct{
	
	store *repository.RedisStore
	policy model.TokenBucketPolicy
}
func NewTokenBucketLimiter(policy model.TokenBucketPolicy,store *repository.RedisStore) *TokenBucketLimiter{
	return &TokenBucketLimiter{
		store: store,
		policy : policy,
	}
}

func(r *TokenBucketLimiter) RateLimit(Clientid string,requestTime time.Time)(model.RateLimitingResponse){
	bucket,err := r.store.GetBucket(context.Background(),Clientid)

	if  err == nil{
		timeDiff :=requestTime.Sub(bucket.LastRefill)
		newTokens :=timeDiff.Seconds() * (bucket.RefillRate)

		if newTokens > 0 {
			bucket.CurrentTokens = min(bucket.CurrentTokens+newTokens,bucket.Capacity)
			bucket.LastRefill=requestTime
		}
			
	}else if errors.Is(err,redis.Nil) {
		bucket.Capacity=r.policy.Capacity
		bucket.CurrentTokens=r.policy.Capacity
		bucket.RefillRate=r.policy.RefillRate
		bucket.LastRefill=requestTime
	}else{
		return model.RateLimitingResponse{}
	}
	var results model.RateLimitingResponse
	if bucket.CurrentTokens >= 1.0 {
			bucket.CurrentTokens-= 1.0
			results.Allowed=true
		}else{
			results.Allowed=false
			seconds := (1-bucket.CurrentTokens) / bucket.RefillRate
			retryAfter := time.Duration(
					seconds *float64(time.Second),
			)
			results.RetryAfter = retryAfter.String()
	}

	if err := r.store.SaveBucket(context.Background(),Clientid,bucket); err != nil{
		return model.RateLimitingResponse{}
	}

	return results
}