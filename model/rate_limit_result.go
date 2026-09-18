package model

type RateLimitResult struct{
	Allowed bool 
	RemianingTokens float64
	RetryAfter float64
}