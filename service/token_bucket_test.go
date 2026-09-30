package service

import (
	"context"
	"testing"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/repository"
)

func TestTokenBucket_AllowRequests(t *testing.T){
	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx) ; err != nil{
		t.Skip("Redis is not running")
	}
	policy := model.TokenBucketPolicy{
		Capacity: 5,
		RefillRate: 1,
	}
	limiter := NewTokenBucketLimiter(policy,store)

	clientId := "test-client-"+time.Now().Format("20060102150405.000000000")

	respose := limiter.RateLimit(
		ctx,
		clientId,
		time.Now(),
	)
	if !respose.Allowed{
		t.Error("expected request to be allowed")
	}
}

func TestTokenBucket_RejectsWhenEmpty(t *testing.T){

	store := repository.NewRedisStore()
	ctx := context.Background()

	if err := store.Ping(ctx) ; err !=nil {
		t.Skip("Redis is not running")
	}

	policy := model.TokenBucketPolicy{
		Capacity: 3,
		RefillRate: 1,
	}

	limiter := NewTokenBucketLimiter(policy,store)

	clientId := "test-client-"+time.Now().Format("20060102150405.000000000")

	requestTime := time.Now()

	for i := 0; i < 3; i++ {
		
		response := limiter.RateLimit(
			ctx,
			clientId,
			requestTime,
		)

		if !response.Allowed{
			t.Fatalf("request %d should have been Allowed",i+1)
		}
	}
	response := limiter.RateLimit(
		ctx,
		clientId,
		requestTime,
	)
	if response.Allowed{
		t.Error("expected request to be rejected when bucket is empty")
	}
}

