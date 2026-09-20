package model

import "time"

type RateLimitingResult struct{
	Allowed bool 
	RetryAfter time.Duration
}