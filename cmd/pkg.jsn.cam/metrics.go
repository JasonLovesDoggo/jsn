// Package main provides metrics for the pkg.jsn.cam server
package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// totalRequests tracks the total number of requests to the server
	totalRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "requests_total",
		Help: "The total number of requests to the pkg.jsn.cam server",
	})

	// requestDuration tracks the duration of requests
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_duration_seconds",
		Help:    "The duration of requests to the pkg.jsn.cam server",
		Buckets: prometheus.DefBuckets,
	}, []string{"path"})

	// repoRequests tracks the number of requests per repository
	repoRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "repo_requests_total",
		Help: "The total number of requests per repository",
	}, []string{"repo"})

	// httpResponseCodes tracks the HTTP response codes
	httpResponseCodes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_response_codes_total",
		Help: "The total number of HTTP response codes",
	}, []string{"code"})

	// goGetRequests tracks the number of go-get=1 requests (actual Go tool downloads)
	goGetRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "goget_requests_total",
		Help: "The total number of go-get=1 requests (actual Go tool downloads)",
	})
)

// MetricsMiddleware wraps an http.Handler and records metrics for each request
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track total requests
		totalRequests.Inc()

		// Track if this is a go-get=1 request (actual Go tool download)
		if r.FormValue("go-get") == "1" {
			goGetRequests.Inc()
		}

		// Extract repository name from path
		path := r.URL.Path
		if path != "/" && path != "" {
			// Remove leading slash and get first path segment
			repo := strings.TrimPrefix(path, "/")
			if idx := strings.Index(repo, "/"); idx > 0 {
				repo = repo[:idx]
			}
			if repo != "" {
				repoRequests.WithLabelValues(repo).Inc()
			}
		}

		// Create a response writer wrapper to capture the status code
		wrw := newResponseWriterWrapper(w)

		// Track request duration
		start := time.Now()
		next.ServeHTTP(wrw, r)
		duration := time.Since(start).Seconds()

		// Record metrics
		requestDuration.WithLabelValues(path).Observe(duration)
		httpResponseCodes.WithLabelValues(wrw.statusCode()).Inc()
	})
}

// responseWriterWrapper is a wrapper for http.ResponseWriter that captures the status code
type responseWriterWrapper struct {
	http.ResponseWriter
	code int
}

// newResponseWriterWrapper creates a new responseWriterWrapper
func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{ResponseWriter: w, code: http.StatusOK}
}

// WriteHeader captures the status code and passes it to the wrapped ResponseWriter
func (rww *responseWriterWrapper) WriteHeader(code int) {
	rww.code = code
	rww.ResponseWriter.WriteHeader(code)
}

// statusCode returns the captured status code as a string
func (rww *responseWriterWrapper) statusCode() string {
	return http.StatusText(rww.code)
}

// RegisterMetricsHandler starts a separate HTTP server for metrics
func RegisterMetricsHandler(port string, lg *slog.Logger) {
	// Create a new mux for metrics
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	// Start the metrics server in a goroutine
	go func() {
		lg.Info("starting metrics server", "port", port, "path", "/metrics")
		err := http.ListenAndServe(":"+port, metricsMux)
		if err != nil {
			lg.Error("metrics server failed", "err", err)
		}
	}()
}
