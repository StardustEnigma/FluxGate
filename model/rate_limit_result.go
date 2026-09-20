package model

type RateLimitingResponse struct{
	Allowed bool `json:"allowed"`
	RetryAfter string `json:"retry_after"`
}