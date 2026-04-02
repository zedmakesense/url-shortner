package app

import (
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/rs/cors"
	"github.com/zedmakesense/url-shortner/backend/internal/handler"
	"github.com/zedmakesense/url-shortner/backend/internal/service"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func NewRouter(service service.ServiceInterface, log *slog.Logger, mail *resend.Client) http.Handler {
	mux := http.NewServeMux()

	h := handler.NewHandler(service, log, mail)

	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/verify-email", h.VerifyEmail)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", h.ForgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", h.ResetPassword)
	// mux.HandleFunc("GET /api/v1/auth/me", h.Me)

	// mux.HandleFunc("GET /{slug}", h.Redirect)
	// mux.HandleFunc("POST /api/v1/urls", h.Urls)
	// mux.HandleFunc("GET /api/v1/urls/{slug}", h.GetURL)
	// mux.HandleFunc("DELETE /api/v1/urls/{slug}", h.DeleteURL)

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			return
		}
	})

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	return loggingMiddleware(log, c.Handler(mux))
}

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		log.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
