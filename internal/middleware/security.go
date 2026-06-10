package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// SecurityHeaders adds security headers to HTTP responses
// to protect against common web vulnerabilities.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking attacks by denying iframe embedding
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME-sniffing attacks by forcing browser to respect declared content-type
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS protection filter in browser
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy controls how much referrer information is sent with requests
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy allows you to restrict the resources (JavaScript, CSS, Images, etc.) that the browser is allowed to load
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		next.ServeHTTP(w, r) // Proceed to the next handler
	})
}

// RequestTimeout adds a timeout context to requests,
// ensuring they do not run indefinitely.
func RequestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout) // Create context with timeout
			defer cancel()                                           // Ensure the cancellation function is called to release resources

			next.ServeHTTP(w, r.WithContext(ctx)) // Serve with new context
		})
	}
}

// rateLimitExemptPaths are infrastructure endpoints that must never be rate
// limited: Kubernetes liveness/readiness probes and Prometheus scrapes. The
// limiter is a single global bucket, so without this exemption a burst of API
// traffic could starve /readyz and /healthz of tokens — returning 429 to the
// kubelet, which would mark the pod unhealthy and restart it. This matters more
// in multi-OLT deployments where one instance fronts many OLTs (higher traffic).
var rateLimitExemptPaths = map[string]struct{}{
	"/health":  {},
	"/healthz": {},
	"/ready":   {},
	"/readyz":  {},
	"/metrics": {},
}

// RateLimiter creates a rate limiting middleware
// params:
// tokensPerSecond: number of requests allowed per second
// burst: maximum burst size (concurrent requests)
// Logs rate limit violations for Prometheus/Grafana/Loki monitoring.
// Infrastructure endpoints (health/readiness probes, metrics) bypass the limiter
// so monitoring and orchestration never see 429s under API load.
func RateLimiter(tokensPerSecond int, burst int) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Limit(tokensPerSecond), burst) // Initialize rate limiter

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, exempt := rateLimitExemptPaths[r.URL.Path]; exempt {
				next.ServeHTTP(w, r) // Probes/metrics are never rate limited
				return
			}
			if !limiter.Allow() { // Check if the request is allowed
				// Log as WARN - rate limit hit (important for monitoring DDoS/abuse patterns)
				logger.WithRequestID(r.Context()).Warn("rate_limit_exceeded",
					zap.String("remote_addr", r.RemoteAddr),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("user_agent", r.UserAgent()),
					zap.Int("limit_per_second", tokensPerSecond),
					zap.Int("burst", burst),
				)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests) // Return 429 Too Many Requests
				return
			}

			next.ServeHTTP(w, r) // Proceed to the next handler
		})
	}
}

// MaxBodySize limits the size of request bodies to prevent large payloads
// from exhausting server memory.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes) // Wrap body reader with MaxBytesReader
			next.ServeHTTP(w, r)                              // Proceed to the next handler
		})
	}
}
