package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	RequestsAllowed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_requests_allowed_total",
			Help: "Total number of allowed requests",
		},
		[]string{"route", "algorithm"},
	)
	RequestsDenied = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_requests_denied_total",
			Help: "Total number of denied requests",
		},
		[]string{"route", "algorithm"},
	)
	CheckDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ratelimiter_check_duration_seconds",
			Help:    "Duration of rate limit checks",
			Buckets: prometheus.DefBuckets,
		},
	)
	CircuitState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ratelimiter_circuit_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"algorithm"},
	)
)

func Handler() http.Handler {
	return promhttp.Handler()
}
