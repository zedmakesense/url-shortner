package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/service"
	"github.com/zedmakesense/url-shortner/internal/utils"
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
	handlerLogger := h.log.With("component", "handler")
	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid json in Register", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if !utils.IsValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid email in Register")
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
	}

	userID, err := h.service.UserCreate(r.Context(), userRequest.Email, userRequest.Name, userRequest.Password)
	if err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
			if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "email already exist"}); encErr != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

}
