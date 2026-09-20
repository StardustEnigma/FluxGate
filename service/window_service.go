package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

type SlidingWindowLimiter struct{
	mu sync.Mutex
	requests map[string][]time.Time
	policy model.SlidingWindowPolicy
}

func NewSlidingWindowLimiter(policy model.SlidingWindowPolicy) *SlidingWindowLimiter{
	return &SlidingWindowLimiter{
		requests: map[string][]time.Time{},
		policy: policy,
	}
}


func(r *SlidingWindowLimiter)RateLimit(Clientid string,requestTime time.Time)(model.RateLimitingResult){
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := r.requests[Clientid]
	var result model.RateLimitingResult
		windowStart := requestTime.Add(-r.policy.TimeWindow)
		validRequest :=requests[:0]
		
		for _,requestTime:=range requests{
			if requestTime.After(windowStart){
				validRequest = append(validRequest, requestTime)
			}
		}
		if len(validRequest) >= r.policy.Limit{
			r.requests[Clientid]=validRequest
			retryAfter := validRequest[0].Add(r.policy.TimeWindow).Sub(requestTime)
			fmt.Println(requestTime.Format("15:04:05"), "Request cancelled:", Clientid)
			result.Allowed = false
			result.RetryAfter = retryAfter
			return result
		}
		validRequest = append(validRequest, requestTime)
		r.requests[Clientid]=validRequest
		fmt.Println(requestTime.Format("15:04:05"), "Request allowed:", Clientid)
		result.Allowed =true
		return result
	
}