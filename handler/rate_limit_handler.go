package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/dto"
	"github.com/StardustEnigma/FluxGate/service"
)

type RateLimiterHandler struct {
	RateLimiter service.RateLimiter
}

func (h *RateLimiterHandler) RateLimit(w http.ResponseWriter, r *http.Request) {
	var req dto.RateLimitRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rateLimitResult, err := h.RateLimiter.RateLimit(
		r.Context(),
		req.ClientId,
		time.Now(),
	)
	if err != nil {
		log.Printf("rate limiter failed: %v", err)
		http.Error(w, "internal rate limiting error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rateLimitResult); err != nil {
		log.Printf("failed to encode rate limit response: %v", err)
	}
}
