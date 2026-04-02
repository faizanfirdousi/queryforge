package middleware

import (
	"log"
	"net/http"
	"time"
)

// Timer logs the method, path, status code, and total duration for every request.
// This is how you spot slow endpoints without a full APM tool.
func Timer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter so we can capture the status code
		wrapped := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.status,
			duration,
		)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the written status code.
// The default ResponseWriter doesn't expose this after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
