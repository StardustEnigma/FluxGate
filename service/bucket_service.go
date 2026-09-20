package service

import (

	"sync"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

type TokenBucketLimiter struct{
	mu sync.Mutex
	buckets map[string]model.Bucket
	policy model.TokenBucketPolicy
}
func NewTokenBucketLimiter(policy model.TokenBucketPolicy) *TokenBucketLimiter{
	return &TokenBucketLimiter{
		buckets : make(map[string]model.Bucket),
		policy : policy,
	}
}

func(r *TokenBucketLimiter) RateLimit(Clientid string,requestTime time.Time)(model.RateLimitingResponse){
	r.mu.Lock()

	defer r.mu.Unlock()
	bucket,ok := r.buckets[Clientid]

	if ok {
		timeDiff :=requestTime.Sub(bucket.LastRefill)
		newTokens :=timeDiff.Seconds() * (bucket.RefillRate)

		if newTokens > 0 {
			bucket.CurrentTokens = min(bucket.CurrentTokens+newTokens,bucket.Capacity)
			bucket.LastRefill=requestTime
		}
			
	}else{
		bucket.Capacity=r.policy.Capacity
		bucket.CurrentTokens=r.policy.Capacity
		bucket.RefillRate=r.policy.RefillRate
		bucket.LastRefill=requestTime
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

	r.buckets[Clientid] =bucket
	return results
}