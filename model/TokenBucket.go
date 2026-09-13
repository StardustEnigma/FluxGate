package model

import "time"

type Bucket struct{
	Capacity float64 `json:"capacity"`
	RefillRate float64 `json:"rate"`
	CurrentTokens float64 `json:"currentTokens"`
	LastRefill time.Time `json:"lastRefill"`
}