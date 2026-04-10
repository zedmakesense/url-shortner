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

const (
	CORSMaxAge = 300
)

type responseWriter struct {
	http.ResponseWriter

	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func NewRouter(service service.Service, log *slog.Logger, mail *resend.Client) http.Handler {
	mux := http.NewServeMux()

	h := handler.NewHandler(service, log, mail)

	// creating user takes name, email and password
	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/verify-email", h.VerifyEmail)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", h.ForgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", h.ResetPassword)
	mux.HandleFunc("GET /{slug}", h.Redirect)

	auth := Auth(h)
	mux.Handle("GET /api/v1/auth/me", auth(http.HandlerFunc(h.Me)))
	mux.Handle("DELETE /api/v1/auth/me", auth(http.HandlerFunc(h.Me)))
	mux.Handle("POST /api/v1/urls", auth(http.HandlerFunc(h.InsertURL)))
	mux.Handle("GET /api/v1/urls", auth(http.HandlerFunc(h.GetURLs)))
	mux.Handle("GET /api/v1/urls/{slug}", auth(http.HandlerFunc(h.GetURL)))
	mux.Handle("DELETE /api/v1/urls/{slug}", auth(http.HandlerFunc(h.DeleteURL)))

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
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
		MaxAge:           CORSMaxAge,
	})

	return loggingMiddleware(log, c.Handler(mux))
}
