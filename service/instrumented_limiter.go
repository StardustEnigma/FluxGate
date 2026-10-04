package service

import (
	"context"
	"time"

	"github.com/StardustEnigma/FluxGate/metrics"
	"github.com/StardustEnigma/FluxGate/model"
)

type InstrumentedLimiter struct {
	next    RateLimiter
	metrics *metrics.Metrics
}

func NewInstrumentedLimiter(
	next RateLimiter,
	m *metrics.Metrics,
) *InstrumentedLimiter {
	return &InstrumentedLimiter{
		next:    next,
		metrics: m,
	}
}

func (i *InstrumentedLimiter) RateLimit(
	ctx context.Context,
	clientId string,
	requestTime time.Time,
) (model.RateLimitingResponse, error) {
	start := time.Now()
	defer func() {
		i.metrics.RequestDuration.Observe(float64(time.Since(start).Seconds()))
	}()
	i.metrics.RequestsTotal.Inc()
	result, err := i.next.RateLimit(ctx, clientId, requestTime)
	if err != nil {
		i.metrics.ErrorsTotal.Inc()
		return result, err
	}

	if result.Allowed {
		i.metrics.AllowedTotal.Inc()
	} else {
		i.metrics.RejectedTotal.Inc()
	}
	return result, nil
}
