package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/dto"
	"github.com/StardustEnigma/FluxGate/metrics"
	"github.com/StardustEnigma/FluxGate/service"
)

type RateLimiterHandler struct {
	RateLimiter service.RateLimiter
	Metrics     *metrics.Metrics // Fix #5 – exposes HTTPRequestDuration histogram
}

func (h *RateLimiterHandler) RateLimit(w http.ResponseWriter, r *http.Request) {
	// Fix #5 – measure the full HTTP handler time so Prometheus reflects what
	// the load-test benchmark actually captures (JSON decode + limiter + JSON
	// encode + TCP flush), not just the inner limiter call.
	handlerStart := time.Now()
	defer func() {
		if h.Metrics != nil {
			h.Metrics.HTTPRequestDuration.Observe(time.Since(handlerStart).Seconds())
		}
	}()

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
