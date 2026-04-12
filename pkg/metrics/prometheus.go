// Package metrics provides Prometheus metrics collection and chi middleware
// for the go-snmp-olt-zte-c320 adapter. It records HTTP request counts,
// durations, in-flight requests, SNMP operation statistics, and cache hit/miss
// counters — everything needed to render adapter dashboards in Grafana.
package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// idPattern matches numeric IDs in URL paths for normalization. This prevents
// high-cardinality labels in Prometheus metrics (every unique board/pon/onu
// combination would otherwise explode the label set).
var idPattern = regexp.MustCompile(`/\d+`)

// Prometheus metric collectors.
var (
	// HTTPRequestsTotal counts total HTTP requests partitioned by method,
	// normalized path, and status code.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration records HTTP request latency in seconds.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight tracks requests currently being processed.
	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed.",
		},
	)

	// SNMPOperationsTotal counts SNMP operations by type and status.
	// `operation` is one of: get, walk, bulkwalk, ping.
	// `status` is one of: success, error.
	SNMPOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmp_operations_total",
			Help: "Total number of SNMP operations.",
		},
		[]string{"operation", "status"},
	)

	// SNMPOperationDuration records SNMP operation latency in seconds.
	SNMPOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "snmp_operation_duration_seconds",
			Help:    "SNMP operation duration in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms → ~10s
		},
		[]string{"operation"},
	)

	// CacheHitsTotal counts Redis cache hits, partitioned by cache key type.
	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmp_cache_hits_total",
			Help: "Total number of SNMP-data cache hits.",
		},
		[]string{"type"},
	)

	// CacheMissesTotal counts Redis cache misses, partitioned by cache key type.
	CacheMissesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "snmp_cache_misses_total",
			Help: "Total number of SNMP-data cache misses.",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		SNMPOperationsTotal,
		SNMPOperationDuration,
		CacheHitsTotal,
		CacheMissesTotal,
	)
}

// NormalizePath replaces numeric IDs in URL paths with `:id` to prevent
// high-cardinality metric labels. For example:
//
//	/api/v1/board/1/pon/8/onu/42 → /api/v1/board/:id/pon/:id/onu/:id
//	/api/v1/board/2/pon/16       → /api/v1/board/:id/pon/:id
func NormalizePath(path string) string {
	return idPattern.ReplaceAllString(path, "/:id")
}

// skipMetricPaths are endpoints excluded from per-request metrics.
// Health and metrics endpoints would distort the histograms and inflate
// request counters without providing useful signal.
var skipMetricPaths = map[string]struct{}{
	"/health":  {},
	"/healthz": {},
	"/ready":   {},
	"/readyz":  {},
	"/metrics": {},
	"/":        {},
}

// Middleware returns a chi-compatible middleware that records Prometheus
// metrics for each HTTP request: total count, duration, and in-flight gauge.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, skip := skipMetricPaths[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			path := NormalizePath(r.URL.Path)
			method := r.Method

			HTTPRequestsInFlight.Inc()
			defer HTTPRequestsInFlight.Dec()

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := strconv.Itoa(ww.Status())
			duration := time.Since(start).Seconds()

			HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
			HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		})
	}
}

// Handler returns the HTTP handler that serves the Prometheus metrics endpoint.
// Mount it at /metrics without any authentication (Prometheus scrapers typically
// run on the same network and authentication gets in the way).
func Handler() http.Handler {
	return promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)
}

// RecordSNMPOperation records the outcome and duration of a single SNMP
// operation. Call via defer to capture the duration cleanly:
//
//	defer metrics.RecordSNMPOperation("get", time.Now(), &err)
//
// `errp` is a pointer to the error variable in the calling function so the
// deferred call sees the final value when the function returns.
func RecordSNMPOperation(operation string, start time.Time, errp *error) {
	status := "success"
	if errp != nil && *errp != nil {
		status = "error"
	}
	SNMPOperationsTotal.WithLabelValues(operation, status).Inc()
	SNMPOperationDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
}

// RecordCacheHit increments the cache hit counter for the given cache type.
func RecordCacheHit(cacheType string) {
	CacheHitsTotal.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss increments the cache miss counter for the given cache type.
func RecordCacheMiss(cacheType string) {
	CacheMissesTotal.WithLabelValues(cacheType).Inc()
}
