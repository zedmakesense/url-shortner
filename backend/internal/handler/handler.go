package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/service"
)

type errorResponse struct {
	Error string `json:"error"`
}

type Handler struct {
	service service.Service
	log     *slog.Logger
	mail    *resend.Client
}

const (
	AccessTokenDuration      = 15 * time.Minute
	RefreshTokenDuration     = 7 * 24 * time.Hour
	AccessTokenCookieMaxAge  = 15 * 60
	RefreshTokenCookieMaxAge = 7 * 24 * 60 * 60
)

func NewHandler(service service.Service, log *slog.Logger, mail *resend.Client) *Handler {
	return &Handler{
		service: service,
		log:     log,
		mail:    mail,
	}
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func (h *Handler) StoreCookies(
	w http.ResponseWriter,
	r *http.Request,
	userID int,
	accessToken string,
	refreshToken string,
) error {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	accessExpiresAt := time.Now().Add(AccessTokenDuration)
	refreshExpiresAt := time.Now().Add(RefreshTokenDuration)

	if err := h.service.StoreTokens(
		r.Context(),
		userID,
		accessToken,
		refreshToken,
		accessExpiresAt,
		refreshExpiresAt,
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "StoreTokens", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return err
		}
		return err
	}

	return nil
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

func (h *Handler) GenerateToken(w http.ResponseWriter, r *http.Request) (string, error) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	token, err := h.service.GenerateToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "access token generation failed", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return "", err
		}
		return "", err
	}

	return token, nil
}

func (h *Handler) parseRegister(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid json in Register", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", "", false
		}
		return "", "", "", false
	}

	if userRequest.Name == "" || userRequest.Email == "" || userRequest.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid json in Register")

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", "", false
		}
		return "", "", "", false
	}

	if !isValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", "", false
		}
		return "", "", "", false
	}

	return userRequest.Email, userRequest.Name, userRequest.Password, true
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	email, name, password, ok := h.parseRegister(w, r)
	if !ok {
		return
	}

	userID, err := h.service.Register(r.Context(), email, name, password)
	if errors.Is(err, domain.ErrEmailAlreadyExists) {
		w.WriteHeader(http.StatusConflict)
		handlerLogger.WarnContext(r.Context(), "email already exist", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "email already exist"}); encErr != nil {
			return
		}
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.WarnContext(r.Context(), "user creation failed", "error", err)

		if !errors.Is(err, domain.ErrCachingFailed) {
			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
				return
			}
		}
		return
	}

	accessToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	refreshToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	if err := h.StoreCookies(w, r, userID, accessToken, refreshToken); err != nil {
		return
	}

	h.WriteCookies(w, accessToken, refreshToken)
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful"}); encErr != nil {
		return
	}
}

func (h *Handler) parseLogin(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid request body", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", false
		}
		return "", "", false
	}

	if userRequest.Password == "" || userRequest.Email == "" {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", false
		}
		return "", "", false
	}

	if !isValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return "", "", false
		}
		return "", "", false
	}

	return userRequest.Email, userRequest.Password, true
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	email, password, ok := h.parseLogin(w, r)
	if !ok {
		return
	}

	userID, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			w.WriteHeader(http.StatusUnauthorized)
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

	accessToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	refreshToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	if err := h.StoreCookies(w, r, userID, accessToken, refreshToken); err != nil {
		return
	}

	h.WriteCookies(w, accessToken, refreshToken)
	w.WriteHeader(http.StatusCreated)

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Login successful"}); encErr != nil {
		return
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		handlerLogger.WarnContext(r.Context(), "cookie", "error", err)
		return
	}

	refreshToken := cookie.Value
	if err = h.service.RevokeToken(r.Context(), refreshToken); err != nil {
		handlerLogger.ErrorContext(r.Context(), "RevokeToken", "error", err)
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	h.WriteCookies(w, "", "")

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"}); encErr != nil {
		return
	}
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		handlerLogger.WarnContext(r.Context(), "cookie", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"}); encErr != nil {
			return
		}
		return
	}

	refreshTokenOld := cookie.Value
	sessionID, userID, err := h.service.GetByRefreshToken(r.Context(), refreshTokenOld)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			w.WriteHeader(http.StatusUnauthorized)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"}); encErr != nil {
				return
			}
			handlerLogger.ErrorContext(r.Context(), "GetByRefreshToken", "error", err)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByRefreshToken", "error", err)
		return
	}

	accessToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	refreshToken, err := h.GenerateToken(w, r)
	if err != nil {
		return
	}

	if err := h.StoreCookies(w, r, userID, accessToken, refreshToken); err != nil {
		return
	}

	if err = h.service.RevokeTokens(r.Context(), userID, sessionID); err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			w.WriteHeader(http.StatusUnauthorized)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "unauthorized"}); encErr != nil {
				return
			}
			handlerLogger.ErrorContext(r.Context(), "RevokeTokens", "error", err)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "RevokeTokens", "error", err)
		return
	}

	h.WriteCookies(w, accessToken, refreshToken)
	resp := map[string]string{"message": "Token refreshed successfully"}

	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}

	if err := h.service.VerifyEmail(r.Context(), token); err != nil {
		if errors.Is(err, domain.ErrEmailVerificationFailed) {
			w.WriteHeader(http.StatusInternalServerError)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "Verification failed"}); encErr != nil {
				return
			}
			handlerLogger.ErrorContext(r.Context(), "VerifyEmail", "error", err)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "VerifyEmail", "error", err)
		return
	}

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "verification successfull"}); encErr != nil {
		return
	}
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	var userRequest domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&userRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid request body", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}

	if !isValidEmail(userRequest.Email) {
		w.WriteHeader(http.StatusBadRequest)
		handlerLogger.WarnContext(r.Context(), "invalid email in Register")

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid request body"}); encErr != nil {
			return
		}
		return
	}

	if err := h.service.SendForgotPasswordMail(r.Context(), userRequest.Email); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "SendForgotPasswordMail", "error", err)

		resp := map[string]string{"message": "forgot password email successfully sended"}
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			return
		}
		return
	}

	resp := map[string]string{"message": "forgot password email successfully sended"}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "missing token"}); encErr != nil {
			return
		}
		return
	}

	userID, err := h.service.VerifyEmailToken(r.Context(), token)
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
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}

	if user.Password == "" {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}

	if err = h.service.ChangePasswordAndRevoke(r.Context(), userID, user.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "ChangePasswordAndRevoke", "error", err)
		return
	}

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "password reset successfull"}); encErr != nil {
		return
	}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}

	user, err := h.service.GetUserByUserID(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetUserByUserID", "error", err)
		return
	}

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
		return
	}
}

func (h *Handler) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	return h.service.ValidateAccessToken(ctx, accessToken)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	shortCode := r.PathValue("slug")
	longURL, err := h.service.GetLongURL(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, domain.ErrURLDoesNotExist) {
			http.NotFound(w, r)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.WarnContext(r.Context(), "GetLongURL", "error", err)
		return
	}

	if err = h.service.URLClicked(r.Context(), shortCode); err != nil {
		if errors.Is(err, domain.ErrURLDoesNotExist) {
			http.NotFound(w, r)
			return
		}
		handlerLogger.WarnContext(r.Context(), "URLClicked", "error", err)
	}

	http.Redirect(w, r, longURL, http.StatusMovedPermanently)
}

func (h *Handler) InsertURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
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
	shortCode.ShortCode, err = h.service.InsertURL(r.Context(), longURL.LongURL, userID)
	if err != nil && errors.Is(err, domain.ErrCachingFailed) {
		if errors.Is(err, domain.ErrURLAlreadyExist) {
			w.WriteHeader(http.StatusBadRequest)
			handlerLogger.WarnContext(r.Context(), "InsertURL", "error", err)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "URL already exist"}); encErr != nil {
				return
			}
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "InsertURL", "error", err)
		return
	}

	if errors.Is(err, domain.ErrCachingFailed) {
		handlerLogger.ErrorContext(r.Context(), "InsertURL", "error", err)
	}

	if encErr := json.NewEncoder(w).Encode(shortCode.ShortCode); encErr != nil {
		return
	}
}

func (h *Handler) GetURLs(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}

	urls, err := h.service.GetURLByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrURLAlreadyExist) {
			w.WriteHeader(http.StatusBadRequest)
			handlerLogger.WarnContext(r.Context(), "URLClicked", "error", err)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "URL already exist"}); encErr != nil {
				return
			}
			return
		}

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

	if err = json.NewEncoder(w).Encode(urlResponses); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	shortCode := r.PathValue("slug")
	url, err := h.service.GetURLByShortCode(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, domain.ErrURLAlreadyExist) {
			w.WriteHeader(http.StatusBadRequest)
			handlerLogger.WarnContext(r.Context(), "URLClicked", "error", err)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "URL already exist"}); encErr != nil {
				return
			}
			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetURLByShortCode", "error", err)
		return
	}

	urlResponse := domain.URLResponse(url)
	if err = json.NewEncoder(w).Encode(urlResponse); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "json encoding", "error", err)
		return
	}
}

func (h *Handler) DeleteURL(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	shortCode := r.PathValue("slug")
	err := h.service.DeleteURLByShortCode(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, domain.ErrURLDoesNotExist) {
			w.WriteHeader(http.StatusInternalServerError)
			handlerLogger.WarnContext(r.Context(), "DeleteURLByShortCode", "error", err)

			if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "URL does not exist"}); encErr != nil {
				return
			}
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		handlerLogger.ErrorContext(r.Context(), "DeleteURLByShortCode", "error", err)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "internal server error"}); encErr != nil {
			return
		}
		return
	}

	resp := map[string]string{"message": "long url deleted successfully"}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		return
	}
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	handlerLogger := h.log.With("component", "handler")
	w.Header().Set("Content-Type", "application/json")

	var user domain.UserRequest
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}

	if user.Password == "" {
		w.WriteHeader(http.StatusBadRequest)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "invalid JSON"}); encErr != nil {
			return
		}
		return
	}

	cookie, _ := r.Cookie("access_token")
	_, userID, err := h.service.GetByAccessToken(r.Context(), cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "GetByAccessToken", "error", err)
		return
	}

	if err = h.service.CheckPassword(r.Context(), userID, user.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "CheckPassword", "error", err)
		return
	}

	if err = h.service.DeleteUser(r.Context(), userID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		if encErr := json.NewEncoder(w).Encode(errorResponse{Error: "password reset failed"}); encErr != nil {
			return
		}
		handlerLogger.ErrorContext(r.Context(), "DeleteUser", "error", err)
		return
	}

	if encErr := json.NewEncoder(w).Encode(map[string]string{"message": "user deleted successfully"}); encErr != nil {
		return
	}
}
