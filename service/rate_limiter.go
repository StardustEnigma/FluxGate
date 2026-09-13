package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

type RateLimiter struct{
	mu sync.Mutex
	Map map[string]model.Bucket
}
func NewRateLimiter() *RateLimiter{
	return &RateLimiter{
		Map : make(map[string]model.Bucket),
	}
}
type RateLimitingService interface{
	RateLimiting(Clientid string,requestTime time.Time)(string)
}
func(r *RateLimiter) RateLimiting(Clientid string,requestTime time.Time)(string){
	r.mu.Lock()

	defer r.mu.Unlock()
	bucket,ok := r.Map[Clientid]

	if ok {
		timeDiff :=requestTime.Sub(bucket.LastRefill)
		newTokens :=timeDiff.Seconds() * (bucket.RefillRate)

		if newTokens > 0 {
			bucket.CurrentTokens = min(bucket.CurrentTokens+newTokens,bucket.Capacity)
			bucket.LastRefill=requestTime
		}
			
		if bucket.CurrentTokens >= 1.0 {
			bucket.CurrentTokens-= 1.0
			fmt.Println("Request Allowed")
		}else{
			fmt.Println("Request rejected")
		}
		
	}else{
		bucket.Capacity=10
		bucket.CurrentTokens=9
		bucket.RefillRate=2
		bucket.LastRefill=requestTime
		fmt.Println("Request Allowed")
	}
	r.Map[Clientid] =bucket
	return "done"
}