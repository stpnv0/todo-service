package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter - обертка над http.ResponseWriter для перехватки статуса
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	return n, err
}

// LoggingMiddleware подробно логгирует запросы к сервису
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}
			start := time.Now()
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			level := slog.LevelInfo
			if rw.status >= 500 {
				level = slog.LevelError
			} else if rw.status >= 400 {
				level = slog.LevelWarn
			} else if rw.status >= 300 {
				level = slog.LevelDebug
			}

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("query", r.URL.RawQuery),
				slog.Int("status", rw.status),
				slog.String("ip", r.RemoteAddr),
				slog.Float64("duration_ms", float64(duration.Milliseconds())),
			}

			logger.Log(r.Context(), level, "http request", attrs...)
		})
	}
}
