package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/handler"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/service"
	"github.com/go-chi/chi/v5"
)

func main(){
	
	
	var limiter service.RateLimiter
	algorithm := flag.String("algorithm","","Rate Limiting Algorithm")
	
	capacity := flag.Float64("capacity",10,"Token Bucket Capacity")
	refillRate := flag.Float64("refill-rate",2,"Token Bucket Refill Rate")

	limit := flag.Int("limit",10,"Sliding Window Request Limit")
	timeWindow := flag.Duration("window",60*time.Second,"Sliding Window")

	if *capacity <= 0 || *refillRate <= 0 {
    	log.Fatal("capacity and refill-rate must be greater than 0")
	}
	if *limit <= 0 || *timeWindow <= 0 {
    	log.Fatal("limit and window must be greater than 0")
	}

	flag.Parse()
	fmt.Println("algorithm =",*algorithm)

	switch *algorithm{
	case "token-bucket":
		tokenBucketpolicy := model.TokenBucketPolicy{
		Capacity:   *capacity,
		RefillRate: *refillRate,
	}
		limiter = service.NewTokenBucketLimiter(tokenBucketpolicy)
	
	case "sliding-window":
		slidingWindowPolicy := model.SlidingWindowPolicy{
		Limit: *limit,
		TimeWindow: *timeWindow,
	}
		limiter = service.NewSlidingWindowLimiter(slidingWindowPolicy)
	
	default :
		log.Fatalf("unknown algorithm : %s",*algorithm)

	}
	rateLimiterHandler := &handler.RateLimiterHandler{RateLimiter: limiter}
	
	r := chi.NewRouter()
	r.Get("/rate-limit",rateLimiterHandler.RateLimit)
	log.Fatal(http.ListenAndServe(":8080",r))
}
