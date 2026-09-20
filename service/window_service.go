package service

import (
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


func(r *SlidingWindowLimiter)RateLimit(Clientid string,requestTime time.Time)(model.RateLimitingResponse){
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := r.requests[Clientid]
	var result model.RateLimitingResponse
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
			result.Allowed = false
			result.RetryAfter = retryAfter.String()
			return result
		}
		validRequest = append(validRequest, requestTime)
		r.requests[Clientid]=validRequest

		result.Allowed =true
		return result
	
}