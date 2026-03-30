package handler

import (
	"log/slog"
	"net/http"

	"github.com/zedmakesense/url-shortner/internal/service"
)

type Handler struct {
	service service.ServiceInterface
	log     *slog.Logger
}

func NewHandler(service service.ServiceInterface, log *slog.Logger) *Handler {
	return &Handler{
		service: service,
		log:     log,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
}
