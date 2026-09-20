package service

import (
	"fmt"
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

func(r *TokenBucketLimiter) RateLimit(Clientid string,requestTime time.Time)(model.RateLimitingResult){
	r.mu.Lock()

	defer r.mu.Unlock()
	bucket,ok := r.buckets[Clientid]
	allowed := false
	var retryAfter time.Duration
	if ok {
		timeDiff :=requestTime.Sub(bucket.LastRefill)
		newTokens :=timeDiff.Seconds() * (bucket.RefillRate)

		if newTokens > 0 {
			bucket.CurrentTokens = min(bucket.CurrentTokens+newTokens,bucket.Capacity)
			bucket.LastRefill=requestTime
		}
			
	}else{
		bucket.Capacity=10
		bucket.CurrentTokens=10
		bucket.RefillRate=2
		bucket.LastRefill=requestTime
	}

	if bucket.CurrentTokens >= 1.0 {
			bucket.CurrentTokens-= 1.0
			allowed=true
		}else{
			retryAfter = time.Duration(((1.0-bucket.CurrentTokens)/bucket.RefillRate) * float64(time.Second))
	}

	r.buckets[Clientid] =bucket

	var results model.RateLimitingResult

	results.Allowed=allowed
	results.RetryAfter=retryAfter
	fmt.Println(results)
	return results
}