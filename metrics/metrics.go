package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Resgistry       *prometheus.Registry
	RequestsTotal   prometheus.Counter
	AllowedTotal    prometheus.Counter
	RejectedTotal   prometheus.Counter
	ErrorsTotal     prometheus.Counter
	RequestDuration prometheus.Histogram
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

		RequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "fluxgate_request_duration_seconds",
			Help:    "Duration of rate-limiting decisions in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	m.Resgistry = prometheus.NewRegistry()
	m.Resgistry.MustRegister(
		m.RequestsTotal,
		m.AllowedTotal,
		m.RejectedTotal,
		m.ErrorsTotal,
		m.RequestDuration,
	)
	return m
}
