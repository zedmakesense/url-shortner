package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

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
			handlerLogger.ErrorContext(r.Context(), "email already exist", "error", err)
			if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "email already exist"}); encErr != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "user creation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	accessToken, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	refreshToken, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	now := time.Now().UTC()
	accessExpiresAt := now.Add(15 * time.Minute)
	refreshExpiresAt := now.Add(7 * 24 * time.Hour)
	if err := h.service.StoreTokens(r.Context(), userID, accessToken, refreshToken, accessExpiresAt, refreshExpiresAt); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	secure := false
	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   15 * 60,
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful"})
}
