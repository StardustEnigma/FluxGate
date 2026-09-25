package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/dto"
	"github.com/StardustEnigma/FluxGate/service"
)

type RateLimiterHandler struct{
	RateLimiter service.RateLimiter
}

func (h *RateLimiterHandler)RateLimit(w http.ResponseWriter,r *http.Request){

	var req dto.RateLimitRequest

	if err := json.NewDecoder(r.Body).Decode(&req);err != nil {
		 http.Error(w, "invalid request body", http.StatusBadRequest)
    	return
	}
	rateLimitResult := h.RateLimiter.RateLimit(req.ClientId,time.Now())
	json.NewEncoder(w).Encode(rateLimitResult)
}