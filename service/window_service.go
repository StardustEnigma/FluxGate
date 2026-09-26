package service

import (
	"context"
	"strconv"
	"time"
	 "github.com/google/uuid"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/redis/go-redis/v9"
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
    windowStart := requestTime.Add(-r.policy.TimeWindow)

    err := r.store.ZRemRangeByScore(
        ctx,
        key,
        "-inf",
        strconv.FormatInt(windowStart.UnixNano(), 10),
    )
    if err != nil {
        return model.RateLimitingResponse{}
    }

    count, err := r.store.ZCard(ctx, key)
    if err != nil {
        return model.RateLimitingResponse{}
    }

    if count >= int64(r.policy.Limit) {

        oldest, err := r.store.ZRangeWithScores(ctx, key, 0, 0)
        if err != nil || len(oldest) == 0 {
            return model.RateLimitingResponse{}
        }

        oldestTime := time.Unix(0, int64(oldest[0].Score))

        retryAfter := oldestTime.
            Add(r.policy.TimeWindow).
            Sub(requestTime)

        return model.RateLimitingResponse{
            Allowed:    false,
            RetryAfter: retryAfter.String(),
        }
    }
    err = r.store.ZAdd(
        ctx,
        key,
        redis.Z{
            Score:  float64(requestTime.UnixNano()),
            Member: uuid.NewString(),
        },
    )
    if err != nil {
        return model.RateLimitingResponse{}
    }

    return model.RateLimitingResponse{
        Allowed: true,
    }
}