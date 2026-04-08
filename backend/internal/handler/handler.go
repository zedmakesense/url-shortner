package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/service"
	"github.com/zedmakesense/url-shortner/internal/utils"
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
	if userRequest.Name == "" || userRequest.Email == "" || userRequest.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid json in Register")
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
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	refreshToken, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "refresh token generation failed", "error", err)
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
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
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
	handlerLogger := h.log.With("component", "handler")
	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if userRequest.Password == "" || userRequest.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if !utils.IsValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid request body")
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
	}

	userID, err := h.service.Login(r.Context(), userRequest.Email, userRequest.Password)

	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			w.WriteHeader(http.StatusConflict)
			handlerLogger.ErrorContext(r.Context(), "User does not exist", "error", err)
			if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "user does not exist"}); encErr != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "service Login err", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	accessToken, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	refreshToken, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "refresh generation token failed", "error", err)
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
	handlerLogger := h.log.With("component", "handler")
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		handlerLogger.ErrorContext(r.Context(), "error in revoking token in logout", "error", err)
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
		handlerLogger.ErrorContext(r.Context(), "sessionId something err:", "error", err)
		return
	}

	accessToken, err := h.service.GenerateToken()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
		return
	}

	refreshToken, err := h.service.GenerateToken()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal error"})
		handlerLogger.ErrorContext(r.Context(), "refresh token generation failed", "error", err)
		return
	}

	accessExpiresAt := time.Now().Add(15 * time.Minute)
	refreshExpiresAt := time.Now().Add(24 * 7 * time.Hour)
	if err := h.service.ReplaceTokens(r.Context(), accessToken, refreshToken, sessionId, accessExpiresAt, refreshExpiresAt); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Server error"})
		handlerLogger.ErrorContext(r.Context(), "refresh function: replace tokens function", "error", err)
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
	handlerLogger := h.log.With("component", "handler")
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
		handlerLogger.ErrorContext(r.Context(), "verify email", "error", err)
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
		handlerLogger.ErrorContext(r.Context(), "cant send email, cuz I dont want to", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "forgot password email successfully sended"})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
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
		handlerLogger.ErrorContext(r.Context(), "verifing email failed", "error", err)
		return
	}

	var user domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if user.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	cookie, _ := r.Cookie("access_token")
	sessionID, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		handlerLogger.ErrorContext(r.Context(), "getting sessionID failed", "error", err)
		return
	}
	if err := h.service.ChangePasswordAndRevoke(r.Context(), userID, user.Password, sessionID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		handlerLogger.ErrorContext(r.Context(), "changing password failed", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "password reset successfull"})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "some err in get by acces token:", "error", err)
		return
	}
	user, err := h.service.GetUserByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "get user by user id", "error", err)
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
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	longURL, err := h.service.GetLongURL(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "get long url", "error", err)
		return
	}
	h.service.URLClicked(r.Context(), shortCode)
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func (h *Handler) InsertURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "cookie shit", "error", err)
		return
	}

	var longURL domain.LongURL
	if err := json.NewDecoder(r.Body).Decode(&longURL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if longURL.LongURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.ErrorContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(domain.ErrorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	var shortCode domain.ShortCode
	shortCode.ShortCode, err = h.service.InsertURL(r.Context(), longURL.LongURL, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "insert url stuff", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shortCode.ShortCode)
}

func (h *Handler) GetURLs(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "get by access token", "error", err)
		return
	}
	urls, err := h.service.GetURLByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "get url by user id", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(urls); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	urls, err := h.service.GetURLByShortCode(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		handlerLogger.ErrorContext(r.Context(), "get url by short code", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(urls); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "get url by short code", "error", err)
		return
	}
}

func (h *Handler) DeleteURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	err := h.service.DeleteURLByShortCode(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "delete url by short code", "error", err)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "long url deleted successfully"})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	var user domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if user.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		handlerLogger.ErrorContext(r.Context(), "getting sessionID failed", "error", err)
		return
	}
	if err := h.service.CheckPassword(r.Context(), userID, user.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		handlerLogger.ErrorContext(r.Context(), "checking password failed", "error", err)
		return
	}
	if err := h.service.DeleteUser(r.Context(), userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "password reset failed"})
		handlerLogger.ErrorContext(r.Context(), "deleting user failed", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "user deleted successfully"})
}
