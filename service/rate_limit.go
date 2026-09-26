package service

import (
	"context"
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

type RateLimiter interface{
	RateLimit(ctx context.Context,Clientid string, requestTime time.Time)(model.RateLimitingResponse)
}