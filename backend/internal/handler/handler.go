package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/backend/internal/domain"
	"github.com/zedmakesense/url-shortner/backend/internal/service"
	"github.com/zedmakesense/url-shortner/backend/internal/utils"
)

type Handler struct {
	service service.ServiceInterface
	log     *slog.Logger
	mail    *resend.Client
}

func NewHandler(service service.ServiceInterface, log *slog.Logger, mail *resend.Client) *Handler {
	return &Handler{
		service: service,
		log:     log,
		mail:    mail,
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

	userID, err := h.service.Register(r.Context(), userRequest.Email, userRequest.Name, userRequest.Password)
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

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if !utils.IsValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
	}

	userID, err := h.service.Login(r.Context(), userRequest.Email, userRequest.Password)

	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			w.WriteHeader(http.StatusConflict)
			if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "user does not exist"}); encErr != nil {
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
	json.NewEncoder(w).Encode(map[string]string{"message": "Login successful"})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		handlerLogger.ErrorContext(r.Context(), "error with cookie in logout", "error", err)
		return
	}
	refreshToken := cookie.Value
	if err := h.service.RevokeToken(r.Context(), refreshToken); err != nil {
		handlerLogger.ErrorContext(r.Context(), "error in revoking token in logout", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-time.Hour),
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-time.Hour),
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	refreshTokenOld := cookie.Value
	sessionId, err := h.service.GetByRefreshToken(r.Context(), refreshTokenOld)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		log.Printf("Token not valid: %v", err)
		return
	}

	accessToken, err := h.service.GenerateToken()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal error"})
		return
	}

	refreshToken, err := h.service.GenerateToken()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal error"})
		return
	}

	accessExpiresAt := time.Now().Add(15 * time.Minute)
	refreshExpiresAt := time.Now().Add(24 * 7 * time.Hour)
	if err := h.service.ReplaceTokens(r.Context(), accessToken, refreshToken, sessionId, accessExpiresAt, refreshExpiresAt); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server error"})
		return
	}

	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  accessExpiresAt,
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  refreshExpiresAt,
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Token refreshed successfully"})
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}
	if err := h.service.VerifyEmail(r.Context(), token); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Verification failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "verification successfull"})
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
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
	if err := h.service.SendForgotPasswordMail(r.Context(), userRequest.Email); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "forgot password email successfully sended"})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}
	if err := h.service.VerifyEmail(r.Context(), token); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "password reset successfull"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	user, err := h.service.GetUserByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := domain.UserResponse{
		ID:              user.ID,
		Name:            user.Name,
		Email:           user.Email,
		IsEmailVerified: user.IsEmailVerified,
		CreatedAt:       user.CreatedAt,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
}

func (h *Handler) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	return h.service.ValidateAccessToken(ctx, accessToken)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("slug")
	longURL, err := h.service.GetLongURL(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	h.service.URLClicked(r.Context(), shortCode)
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}
