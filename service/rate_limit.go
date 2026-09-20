package service

import (
	"time"

	"github.com/StardustEnigma/FluxGate/model"
)

type RateLimiter interface{
	RateLimit(Clientid string, requestTime time.Time)(model.RateLimitingResponse)
}