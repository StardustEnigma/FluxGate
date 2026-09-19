package model

import "time"

type TokenBucketPolicy struct{
	Capacity float64
	RefillRate float64
}

type SlidingWindowPolicy struct{
	Limit int
	TimeWindow time.Duration
}
