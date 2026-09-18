package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/service"
)

type Handler struct{
	RateLimitService service.RateLimitingService
}

func (h *Handler)Request(w http.ResponseWriter,r *http.Request){

	var Clientid string
	json.NewDecoder(r.Body).Decode(&Clientid)
	
	rateLimitResult := h.RateLimitService.RateLimiting(Clientid,time.Now())
	json.NewEncoder(w).Encode(rateLimitResult)
}