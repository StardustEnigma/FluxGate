package routes

import (
	"github.com/StardustEnigma/FluxGate/handler"
	"github.com/StardustEnigma/FluxGate/model"
	"github.com/StardustEnigma/FluxGate/service"
	"github.com/go-chi/chi/v5"
)

func Routes() *chi.Mux {
	r := chi.NewRouter()
	policy := model.RateLimitPolicy{
		Capacity:   10,
		RefillRate: 2,
	}
	rateLimitService := service.NewRateLimiter(policy)
	handler := &handler.Handler{RateLimitService: rateLimitService}
	r.Get("/api", handler.Request)
	return r
}
