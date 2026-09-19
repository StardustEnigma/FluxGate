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
type RateLimitingService interface{
	TokenBucketLimiting(Clientid string,requestTime time.Time)(model.TokenBucketResult)
}
func(r *TokenBucketLimiter) TokenBucketLimiting(Clientid string,requestTime time.Time)(model.TokenBucketResult){
	r.mu.Lock()

	defer r.mu.Unlock()
	bucket,ok := r.buckets[Clientid]
	allowed := false
	retryAfter:=0.0
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
			retryAfter =(1.0-bucket.CurrentTokens)/bucket.RefillRate
	}

	
	r.buckets[Clientid] =bucket

	var results model.TokenBucketResult

	results.Allowed=allowed
	results.RemianingTokens=bucket.CurrentTokens
	results.RetryAfter=retryAfter
	fmt.Println(results)
	return results
}