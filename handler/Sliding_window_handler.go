package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StardustEnigma/FluxGate/service"
)

type SldingWindowHandler struct{
	SlidingWindowService service.SlidingWindowService
}

func (h *SldingWindowHandler)SlidingWindow(w http.ResponseWriter,r *http.Request){
	var Clientid string
	json.NewDecoder(r.Body).Decode(&Clientid)
	result := h.SlidingWindowService.SlidingWindowLimiting(Clientid,time.Now())
	json.NewEncoder(w).Encode(result)
}