package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var registerTransportMetrics sync.Once

// InstrumentedTransport is middleware that wraps the provided
// http.RoundTripper, adding the default Prometheus http client metrics.
//
// The transport should be re-used between clients or the metrics will get
// lost/re-initialised.
func InstrumentedTransport(t http.RoundTripper) http.RoundTripper {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "storageos_api_in_flight_requests",
		Help: "A gauge of in-flight requests for the api client.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storageos_api_requests_total",
			Help: "A counter for requests from the api client.",
		},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storageos_api_request_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// Make sure that metrics are only registered only once.
	registerTransportMetrics.Do(func() {
		metrics.Registry.MustRegister(counter, histVec, inFlightGauge)
	})

	return promhttp.InstrumentRoundTripperInFlight(inFlightGauge,
		promhttp.InstrumentRoundTripperCounter(counter,
			promhttp.InstrumentRoundTripperDuration(histVec, t),
		),
	)
}
