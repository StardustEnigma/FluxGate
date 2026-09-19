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

type SlidingWindowService interface{
	SlidingWindowLimiting(Clientid string,requestTime time.Time)(bool)
}

func(r *SlidingWindowLimiter)SlidingWindowLimiting(Clientid string,requestTime time.Time)(bool){
	r.mu.Lock()
	defer r.mu.Unlock()
	requests := r.requests[Clientid]

		windowStart := requestTime.Add(-r.policy.TimeWindow)
		validRequest :=requests[:0]

		for _,requestTime:=range requests{
			if requestTime.After(windowStart){
				validRequest = append(validRequest, requestTime)
			}
		}
		if len(validRequest) >= r.policy.Limit{
			r.requests[Clientid]=validRequest
			fmt.Println(requestTime.Format("15:04:05"), "Request cancelled:", Clientid)
			return false
		}
		validRequest = append(validRequest, requestTime)
		r.requests[Clientid]=validRequest
		fmt.Println(requestTime.Format("15:04:05"), "Request allowed:", Clientid)
		return true
	
}