package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/service"
)

type RateLimiterHandler struct{
	RateLimiter service.RateLimiter
}

func (h *RateLimiterHandler)RateLimit(w http.ResponseWriter,r *http.Request){

	var Clientid string
	json.NewDecoder(r.Body).Decode(&Clientid)
	
	rateLimitResult := h.RateLimiter.RateLimit(Clientid,time.Now())
	json.NewEncoder(w).Encode(rateLimitResult)
}