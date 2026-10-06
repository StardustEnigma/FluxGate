package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Resgistry           *prometheus.Registry
	RequestsTotal       prometheus.Counter
	AllowedTotal        prometheus.Counter
	RejectedTotal       prometheus.Counter
	ErrorsTotal         prometheus.Counter
	RequestDuration     prometheus.Histogram // limiter-only (Redis Eval) time
	HTTPRequestDuration prometheus.Histogram // full HTTP handler time (Fix #5)
}

func NewMetrics() *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "fluxgate_requests_total",
				Help: "Total number of rate-limit decisions",
			},
		),
		AllowedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "fluxgate_allowed_total",
				Help: "Total number of allowed requests",
			},
		),
		RejectedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "fluxgate_rejected_total",
				Help: "Total number of rejected requests",
			},
		),
		ErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fluxgate_errors_total",
			Help: "Total number of internal rate-limiting errors.",
		}),

		// Fix #5a – limiter-only histogram (unchanged, measured inside InstrumentedLimiter)
		RequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "fluxgate_request_duration_seconds",
			Help:    "Duration of rate-limiting decisions in seconds (limiter call only).",
			Buckets: prometheus.DefBuckets,
		}),

		// Fix #5b – full HTTP handler histogram (JSON decode + limiter + JSON encode + flush)
		// This matches exactly what the load-test benchmark measures end-to-end.
		HTTPRequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "fluxgate_http_request_duration_seconds",
			Help:    "Full HTTP handler duration in seconds (decode + limiter + encode).",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		}),
	}
	m.Resgistry = prometheus.NewRegistry()
	m.Resgistry.MustRegister(
		m.RequestsTotal,
		m.AllowedTotal,
		m.RejectedTotal,
		m.ErrorsTotal,
		m.RequestDuration,
		m.HTTPRequestDuration,
	)
	return m
}
