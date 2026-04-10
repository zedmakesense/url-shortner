package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/service"
)

type errorResponse struct {
	Error string `json:"error"`
}

type Handler struct {
	userSvc     *service.UserService
	sessionSvc  *service.SessionService
	emailSvc    *service.EmailService
	urlSvc      *service.URLService
	passwordSvc *service.PasswordService
	log         *slog.Logger
}

const (
	AccessTokenDuration      = 15 * time.Minute
	RefreshTokenDuration     = 7 * 24 * time.Hour
	AccessTokenCookieMaxAge  = 15 * 60
	RefreshTokenCookieMaxAge = 7 * 24 * 60 * 60
)

func NewHandler(
	userSvc *service.UserService,
	sessionSvc *service.SessionService,
	emailSvc *service.EmailService,
	urlSvc *service.URLService,
	passwordSvc *service.PasswordService,
	log *slog.Logger,
) *Handler {
	return &Handler{
		userSvc:     userSvc,
		sessionSvc:  sessionSvc,
		emailSvc:    emailSvc,
		urlSvc:      urlSvc,
		passwordSvc: passwordSvc,
		log:         log,
	}
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func (h *Handler) GenerateToken(w http.ResponseWriter, r *http.Request) string {
	handlerLogger := h.log.With("component", "handler")
	token, err := h.sessionSvc.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return ""
		}
		return ""
	}
	return token
}

func (h *Handler) parseRegister(w http.ResponseWriter, r *http.Request) (string, string, string) {
	handlerLogger := h.log.With("component", "handler")

	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid json in Register", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", ""
		}
		return "", "", ""
	}
	if userRequest.Name == "" || userRequest.Email == "" || userRequest.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid json in Register")
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", ""
		}
		return "", "", ""
	}
	if !isValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", ""
		}
	}
	return userRequest.Email, userRequest.Name, userRequest.Password
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	email, name, password := h.parseRegister(w, r)
	userID, err := h.userSvc.Register(r.Context(), email, name, password)
	if err != nil {
		if errors.Is(err, domain.ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
			handlerLogger.WarnContext(r.Context(), "email already exist", "error", err)
			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "email already exist"}); encErr != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.WarnContext(r.Context(), "user creation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	accessToken := h.GenerateToken(w, r)
	refreshToken := h.GenerateToken(w, r)

	now := time.Now().UTC()
	accessExpiresAt := now.Add(AccessTokenDuration)
	refreshExpiresAt := now.Add(RefreshTokenDuration)
	if err = h.sessionSvc.StoreTokens(
		r.Context(),
		userID,
		accessToken,
		refreshToken,
		accessExpiresAt,
		refreshExpiresAt); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
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
		MaxAge:   AccessTokenCookieMaxAge,
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   RefreshTokenCookieMaxAge,
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful"}); encErr != nil {
		return
	}
}

func (h *Handler) parseLogin(w http.ResponseWriter, r *http.Request) (string, string) {
	handlerLogger := h.log.With("component", "handler")
	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", ""
		}
		return "", ""
	}
	if userRequest.Password == "" || userRequest.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", ""
		}
		return "", ""
	}
	if !isValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", ""
		}
	}
	return userRequest.Email, userRequest.Password
}

func (h *Handler) WriteCookies(w http.ResponseWriter, accessToken string, refreshToken string) {
	secure := false
	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   AccessTokenCookieMaxAge,
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   RefreshTokenCookieMaxAge,
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")

	email, password := h.parseLogin(w, r)
	userID, err := h.userSvc.Login(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			w.WriteHeader(http.StatusConflict)
			handlerLogger.ErrorContext(r.Context(), "User does not exist", "error", err)
			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "user does not exist"}); encErr != nil {
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "Login", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	accessToken := h.GenerateToken(w, r)
	refreshToken := h.GenerateToken(w, r)

	now := time.Now().UTC()
	accessExpiresAt := now.Add(AccessTokenDuration)
	refreshExpiresAt := now.Add(RefreshTokenDuration)
	if err = h.sessionSvc.StoreTokens(
		r.Context(),
		userID,
		accessToken,
		refreshToken,
		accessExpiresAt,
		refreshExpiresAt); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}
	h.WriteCookies(w, accessToken, refreshToken)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Login successful"}); encErr != nil {
		return
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		handlerLogger.WarnContext(r.Context(), "cookie", "error", err)
		return
	}
	refreshToken := cookie.Value
	if err = h.sessionSvc.RevokeToken(r.Context(), refreshToken); err != nil {
		handlerLogger.ErrorContext(r.Context(), "RevokeToken", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
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
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"}); encErr != nil {
		return
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		handlerLogger.WarnContext(r.Context(), "cookie", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	refreshTokenOld := cookie.Value
	sessionID, userID, err := h.sessionSvc.GetByRefreshToken(r.Context(), refreshTokenOld)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByRefreshToken", "error", err)
		return
	}

	accessToken := h.GenerateToken(w, r)
	refreshToken := h.GenerateToken(w, r)

	accessExpiresAt := time.Now().Add(AccessTokenDuration)
	refreshExpiresAt := time.Now().Add(RefreshTokenDuration)
	if err = h.sessionSvc.StoreTokens(
		r.Context(),
		userID,
		accessToken,
		refreshToken,
		accessExpiresAt,
		refreshExpiresAt); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "Server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "ReplaceTokens", "error", err)
		return
	}

	if err = h.sessionSvc.RevokeTokens(r.Context(), userID, sessionID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "Server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "RevokeToken", "error", err)
		return
	}

	h.WriteCookies(w, accessToken, refreshToken)
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{"message": "Token refreshed successfully"}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}
	if err := h.emailSvc.VerifyEmail(r.Context(), token); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "Verification failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "VerifyEmail", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "verification successfull"}); encErr != nil {
		return
	}
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp := map[string]string{"message": "forgot password email successfully sended"}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			return
		}
		return
	}
	if !isValidEmail(userRequest.Email) {
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")
		resp := map[string]string{"message": "forgot password email successfully sended"}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			return
		}
		return
	}
	if err := h.emailSvc.SendForgotPasswordMail(r.Context(), userRequest.Email); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "SendForgotPasswordMail", "error", err)
		resp := map[string]string{"message": "forgot password email successfully sended"}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			return
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{"message": "forgot password email successfully sended"}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}
	userID, err := h.emailSvc.VerifyEmailToken(r.Context(), token)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "VerifyEmail", "error", err)
		return
	}

	var user domain.UserRequest
	if err = json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}
	if user.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}

	if err = h.passwordSvc.ChangePasswordAndRevoke(r.Context(), userID, user.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "ChangePasswordAndRevoke", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "password reset successfull"}); encErr != nil {
		return
	}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.sessionSvc.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}
	user, err := h.userSvc.GetUserByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetUserByUserID", "error", err)
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
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
	}
}

func (h *Handler) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	return h.sessionSvc.ValidateAccessToken(ctx, accessToken)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	longURL, err := h.urlSvc.GetLongURL(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.WarnContext(r.Context(), "GetLongURL", "error", err)
		return
	}
	if err = h.urlSvc.URLClicked(r.Context(), shortCode); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.WarnContext(r.Context(), "URLClicked", "error", err)
		return
	}
	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func (h *Handler) InsertURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.sessionSvc.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}

	var longURL domain.LongURL
	if err = json.NewDecoder(r.Body).Decode(&longURL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	if longURL.LongURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid request body", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}
	var shortCode domain.ShortCode
	shortCode.ShortCode, err = h.urlSvc.InsertURL(r.Context(), longURL.LongURL, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "InsertURL", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(shortCode.ShortCode); encErr != nil {
		return
	}
}

func (h *Handler) GetURLs(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.sessionSvc.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}
	urls, err := h.urlSvc.GetURLByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetURLByUserID", "error", err)
		return
	}
	var urlResponses []domain.URLResponse
	for _, url := range urls {
		urlResponses = append(urlResponses, domain.URLResponse(url))
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(urlResponses); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	url, err := h.urlSvc.GetURLByShortCode(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetURLByShortCode", "error", err)
		return
	}
	urlResponse := domain.URLResponse(url)
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(urlResponse); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "json encoding", "error", err)
		return
	}
}

func (h *Handler) DeleteURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	shortCode := r.PathValue("slug")
	err := h.urlSvc.DeleteURLByShortCode(r.Context(), shortCode)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "DeleteURLByShortCode", "error", err)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{"message": "long url deleted successfully"}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	var user domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}
	if user.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}
	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.sessionSvc.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}
	if err = h.userSvc.CheckPassword(r.Context(), userID, user.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "CheckPassword", "error", err)
		return
	}
	if err = h.userSvc.DeleteUser(r.Context(), userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "DeleteUser", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "user deleted successfully"}); encErr != nil {
		return
	}
}
