package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/handler"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
	"github.com/StardustEnigma/FluxGate/service"
	"github.com/go-chi/chi/v5"
)

func main(){
	
	store := repository.NewRedisStore()
	if err := store.Ping(context.Background()); err !=nil {
		log.Fatal(err)
	}
	fmt.Println("Redis Connected")
	
	var limiter service.RateLimiter
	algorithm := flag.String("algorithm","","Rate Limiting Algorithm")
	
	capacity := flag.Float64("capacity",10,"Token Bucket Capacity")
	refillRate := flag.Float64("refill-rate",2,"Token Bucket Refill Rate")

	limit := flag.Int("limit",10,"Sliding Window Request Limit")
	timeWindow := flag.Duration("window",60*time.Second,"Sliding Window")
	flag.Parse()
	if *algorithm =="" {
		log.Fatal("algorithm is required")
	}
	fmt.Println("algorithm =",*algorithm)

	switch *algorithm{
	case "token-bucket":
		if *capacity <=0 {
			log.Fatal("capacity must be greater than 0")
		}
		if *refillRate <=0 {
			log.Fatal("refill rate must be greater than 0")
		}
		tokenBucketpolicy := model.TokenBucketPolicy{
		Capacity:   *capacity,
		RefillRate: *refillRate,
	}
		limiter = service.NewTokenBucketLimiter(tokenBucketpolicy)
	
	case "sliding-window":
		if *limit <= 0 {
			log.Fatal("limit must be greater than 0")
		}
		if *timeWindow <=0 {
			log.Fatal("window must be greater than 0")
		}
		slidingWindowPolicy := model.SlidingWindowPolicy{
		Limit: *limit,
		TimeWindow: *timeWindow,
	}
		limiter = service.NewSlidingWindowLimiter(slidingWindowPolicy)
	
	default :
		log.Fatalf("unknown algorithm : %s (use token-bucket or sliding-window)" ,*algorithm)

	}
	rateLimiterHandler := &handler.RateLimiterHandler{RateLimiter: limiter}
	
	r := chi.NewRouter()
	r.Get("/rate-limit",rateLimiterHandler.RateLimit)
	log.Fatal(http.ListenAndServe(":8080",r))
}
