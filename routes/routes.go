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
	rateLimitService := service.NewTokenBucketLimiter(TokenBucketpolicy)
	slidingLimitService := service.NewSlidingWindowLimiter(SlidingWindowPolicy)
	TokenBucketHandler := &handler.TokenBucketHandler{RateLimitService: rateLimitService}
	SlidingWindowHandler := &handler.SldingWindowHandler{SlidingWindowService: slidingLimitService}
	r.Get("/token-bucket", TokenBucketHandler.TokenBucket)
	r.Get("/sliding-window",SlidingWindowHandler.SlidingWindow)

	return r
}
