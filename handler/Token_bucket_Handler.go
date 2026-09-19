package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/service"
)

type TokenBucketHandler struct{
	RateLimitService service.RateLimitingService
}

func (h *TokenBucketHandler)TokenBucket(w http.ResponseWriter,r *http.Request){

	var Clientid string
	json.NewDecoder(r.Body).Decode(&Clientid)
	
	rateLimitResult := h.RateLimitService.TokenBucketLimiting(Clientid,time.Now())
	json.NewEncoder(w).Encode(rateLimitResult)
}