package service

import (
	"context"
	"time"
	 "github.com/google/uuid"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"

)

type SlidingWindowLimiter struct{
	store *repository.RedisStore
	policy model.SlidingWindowPolicy
}

func NewSlidingWindowLimiter(policy model.SlidingWindowPolicy,store *repository.RedisStore) *SlidingWindowLimiter{
	return &SlidingWindowLimiter{
		store: store,
		policy: policy,
	}
}

func (r *SlidingWindowLimiter) RateLimit(
    ctx context.Context,
    clientID string,
    requestTime time.Time,
) model.RateLimitingResponse {

    key := "rate-limit:window:" + clientID
    result,err := r.store.Eval(
        ctx,
        slidingWindowScript,
        []string{key},
        requestTime.UnixNano(),
        r.policy.TimeWindow.Nanoseconds(),
        r.policy.Limit,
        uuid.NewString(),
    )
    if err != nil {
        return model.RateLimitingResponse{}
    }
    values :=result.([]interface{})

    if values[0].(int64)==1 {
        return model.RateLimitingResponse{
            Allowed: true,
        }
    }
    retryAfter := time.Duration(values[1].(int64))

    return model.RateLimitingResponse{
        Allowed: false,
        RetryAfter: retryAfter.String(),
    }
}