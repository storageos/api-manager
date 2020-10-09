package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// LatencyMetric observes latency of wrappered or composite api calls.
type LatencyMetric interface {
	Observe(function string, latency time.Duration)
}

// ResultMetric counts wrappered function errors.
type ResultMetric interface {
	Increment(function string, err error)
}

var (
	// Latency is the latency metric that wrappered or composite api calls will
	// update.
	Latency LatencyMetric = &latencyAdapter{m: helperLatencyHistogram}

	// Errors counts errors encountered by api helpers.
	Errors ResultMetric = &resultAdapter{m: helperResultCounter}

	// registerMetricsOnce keeps track of metrics registration.
	registerMetricsOnce sync.Once
)

var (
	helperLatencyHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storageos_api_helper_duration_seconds",
			Help:    "Distribution of the length of time api helpers take to complete.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"function"},
	)

	helperResultCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storageos_api_helper_total",
			Help: "Number of api helper calls, partitioned by function name and error string.",
		},
		[]string{"function", "error"},
	)
)

// RegisterMetrics ensures that the package metrics are registered.
func RegisterMetrics() {
	registerMetricsOnce.Do(func() {
		metrics.Registry.MustRegister(helperLatencyHistogram)
		metrics.Registry.MustRegister(helperResultCounter)
	})
}

type latencyAdapter struct {
	m *prometheus.HistogramVec
}

func (l *latencyAdapter) Observe(function string, latency time.Duration) {
	l.m.WithLabelValues(function).Observe(latency.Seconds())
}

type resultAdapter struct {
	m *prometheus.CounterVec
}

func (r *resultAdapter) Increment(function string, err error) {
	if err == nil {
		r.m.WithLabelValues(function, "").Inc()
	} else {
		r.m.WithLabelValues(function, err.Error()).Inc()
	}
}
