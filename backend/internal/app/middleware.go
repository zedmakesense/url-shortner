package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zedmakesense/url-shortner/internal/handler"
)

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := uuid.New().String()

		ctx := context.WithValue(r.Context(), "req_id", reqID)
		r = r.WithContext(ctx)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		w.Header().Set("X-Request-ID", reqID)

		var level slog.Level
		switch {
		case rw.status >= 500:
			level = slog.LevelError
		case rw.status >= 400:
			level = slog.LevelWarn
		default:
			level = slog.LevelInfo
		}

		log.Log(ctx, level, "http_request",
			slog.String("req_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("ip", r.RemoteAddr),
			slog.String("user_agent", r.Header.Get("User-Agent")),
		)
	})
}

func Auth(h *handler.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("access_token")
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				slog.WarnContext(r.Context(), "Token not found in cookie:", "error", err)
				return
			}

			_, _, err = h.ValidateAccessToken(r.Context(), cookie.Value)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				slog.WarnContext(r.Context(), "Token not valid:", "error", err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
