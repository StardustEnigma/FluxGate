package routes

import (
	"time"

	"github.com/StardustEnigma/FluxGate/handler"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/service"
	"github.com/go-chi/chi/v5"
)

func Routes() *chi.Mux {
	r := chi.NewRouter()
	TokenBucketpolicy := model.TokenBucketPolicy{
		Capacity:   10,
		RefillRate: 2,
	}
	SlidingWindowPolicy := model.SlidingWindowPolicy{
		Limit: 10,
		TimeWindow: 60*time.Second,
	}
	tokenBucket := service.NewTokenBucketLimiter(TokenBucketpolicy)
	slindingWindow := service.NewSlidingWindowLimiter(SlidingWindowPolicy)
	SlidingWindowHandler := &handler.RateLimiterHandler{
		RateLimiter : slindingWindow,
	}
	TokenBucketHandler := &handler.RateLimiterHandler{
		RateLimiter: tokenBucket,
	}
	r.Get("/token-bucket", TokenBucketHandler.RateLimit)
    r.Get("/sliding-window", SlidingWindowHandler.RateLimit)

	return r
}
