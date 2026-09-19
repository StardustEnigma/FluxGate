package model

type TokenBucketResult struct{
	Allowed bool 
	RemianingTokens float64
	RetryAfter float64
}