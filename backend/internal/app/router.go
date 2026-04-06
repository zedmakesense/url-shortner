package app

import (
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/resend/resend-go/v3"
	"github.com/rs/cors"
	"github.com/zedmakesense/url-shortner/internal/handler"
	"github.com/zedmakesense/url-shortner/internal/service"
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

	auth := Auth(h)
	mux.Handle("GET /api/v1/auth/me", auth(http.HandlerFunc(h.Me)))
	mux.Handle("GET /{slug}", auth(http.HandlerFunc(h.Redirect)))
	mux.Handle("POST /api/v1/urls", auth(http.HandlerFunc(h.InsertURL)))
	mux.Handle("GET /api/v1/urls", auth(http.HandlerFunc(h.GetURLs)))
	mux.Handle("GET /api/v1/urls/{slug}", auth(http.HandlerFunc(h.GetURL)))
	mux.Handle("DELETE /api/v1/urls/{slug}", auth(http.HandlerFunc(h.DeleteURL)))

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
