package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type RepositoryInterface interface {
	CacheUserProfile(ctx context.Context,
		userID int,
		name string,
		email string,
		isEmailVerified bool,
		createdAt time.Time) error
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
	InsertSession(
		ctx context.Context,
		userID int,
		accessTokenHash []byte,
		refreshTokenHash []byte,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time) error
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetCachedProfile(ctx context.Context, userID int) (domain.User, bool, error)
	GetUserByUserID(ctx context.Context, userID int) (domain.User, error)
	RevokeToken(ctx context.Context, sessionID int) error
	RevokeTokens(ctx context.Context, userID int, sessionID int) error
	GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error)
	GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error)
	ReplaceTokens(
		ctx context.Context,
		accessTokenHash []byte,
		refreshTokenHash []byte,
		sessionID int,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time) error
	GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error)
	GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error)
	RevokeEmailTokens(ctx context.Context, userID int) error
	InsertEmailToken(ctx context.Context, userID int, HashedToken []byte, expiresAt time.Time) error
	MarkUserVerified(ctx context.Context, userID int) error
	ChangePasswordAndRevoke(ctx context.Context, userID int, hashedPassword string) error
	GetCachedLongURL(ctx context.Context, shortCode string) (string, bool, error)
	GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error)
	GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error)
	URLClicked(ctx context.Context, shortCode string) error
	InsertURL(ctx context.Context, shortCode string, longURL string, userID int, createdAt time.Time) error
	CacheShortURL(ctx context.Context, shortCode string, longURL string) error
	DeleteURLByShortCode(ctx context.Context, shortCode string) error
	DeleteUser(ctx context.Context, userID int) error
}

type Service interface {
	Register(ctx context.Context, email string, name string, password string) (int, error)
	StoreTokens(
		ctx context.Context,
		userID int,
		accessToken string,
		refreshToken string,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time) error
	GenerateToken() (string, error)
	Login(ctx context.Context, email string, password string) (int, error)
	RevokeToken(ctx context.Context, refreshToken string) error
	RevokeTokens(ctx context.Context, userID int, sessionID int) error
	ReplaceTokens(
		ctx context.Context,
		accessToken string,
		refreshToken string,
		userID int,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time) error
	GetByRefreshToken(ctx context.Context, refreshToken string) (int, int, error)
	GetByAccessToken(ctx context.Context, accessToken string) (int, int, error)
	GetUserByUserID(ctx context.Context, userID int) (domain.User, error)
	ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error)
	CheckEmail(ctx context.Context, userID int) error
	RevokeEmailTokens(ctx context.Context, userID int) error
	SendEmail(ctx context.Context, email string, userID int, expiresAt int) error
	VerifyEmail(ctx context.Context, token string) error
	VerifyEmailToken(ctx context.Context, token string) (int, error)
	ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error
	SendForgotPasswordMail(ctx context.Context, email string) error
	GetLongURL(ctx context.Context, shortCode string) (string, error)
	URLClicked(ctx context.Context, shortCode string) error
	InsertURL(ctx context.Context, longURL string, userID int) (string, error)
	GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error)
	GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error)
	DeleteURLByShortCode(ctx context.Context, shortCode string) error
	DeleteUser(ctx context.Context, userID int) error
	CheckPassword(ctx context.Context, userID int, password string) error
}

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type serviceStruct struct {
	repo RepositoryInterface
	log  *slog.Logger
	mail *resend.Client
}

func NewService(repo RepositoryInterface, log *slog.Logger, mail *resend.Client) Service {
	return &serviceStruct{
		repo: repo,
		log:  log,
		mail: mail,
	}
}

func (s *serviceStruct) cacheUserProfile(ctx context.Context,
	userID int,
	name string,
	email string,
	isEmailVerified bool,
	createdAt time.Time,
) error {
	return s.repo.CacheUserProfile(ctx, userID, name, email, isEmailVerified, createdAt)
}

func (s *serviceStruct) Register(ctx context.Context, email string, name string, password string) (int, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return 0, err
	}

	userID, err := s.repo.InsertUser(ctx, email, name, hashedPassword)
	if err != nil {
		return 0, err
	}

	if err = s.SendEmail(ctx, email, userID, 1); err != nil {
		return userID, err
	}

	if err = s.cacheUserProfile(ctx, userID, name, email, false, time.Now()); err != nil {
		return userID, domain.ErrCachingFailed
	}

	return userID, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (s *serviceStruct) GenerateToken() (string, error) {
	size := 32
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func (s *serviceStruct) StoreTokens(
	ctx context.Context,
	userID int,
	accessToken string,
	refreshToken string,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	return s.repo.InsertSession(
		ctx,
		userID,
		hashToken(accessToken),
		hashToken(refreshToken),
		accessExpiresAt,
		refreshExpiresAt)
}

func (s *serviceStruct) Login(ctx context.Context, email string, password string) (int, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return 0, err
	}
	return user.ID, comparePassword(user.HashedPassword, password)
}

func (s *serviceStruct) CheckPassword(ctx context.Context, userID int, password string) error {
	user, err := s.repo.GetUserByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return comparePassword(user.HashedPassword, password)
}

func (s *serviceStruct) RevokeToken(ctx context.Context, refreshToken string) error {
	token, err := s.repo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return err
	}
	return s.repo.RevokeToken(ctx, token.SessionID)
}

func (s *serviceStruct) RevokeTokens(ctx context.Context, userID int, sessionID int) error {
	return s.repo.RevokeTokens(ctx, userID, sessionID)
}

func (s *serviceStruct) ReplaceTokens(
	ctx context.Context,
	accessToken string,
	refreshToken string,
	userID int,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	return s.repo.ReplaceTokens(
		ctx,
		hashToken(accessToken),
		hashToken(refreshToken),
		userID, accessExpiresAt, refreshExpiresAt)
}

func (s *serviceStruct) GetByAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	token, err := s.repo.GetByAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return 0, 0, err
	}
	return token.SessionID, token.UserID, nil
}

func (s *serviceStruct) GetUserByUserID(ctx context.Context, userID int) (domain.User, error) {
	user, hit, err := s.repo.GetCachedProfile(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if hit {
		return domain.User{}, err
	}
	user, err = s.repo.GetUserByUserID(ctx, userID)
	go func() {
		_ = s.repo.CacheUserProfile(ctx, userID, user.Name, user.Email, user.IsEmailVerified, user.CreatedAt)
	}()
	return user, err
}

func (s *serviceStruct) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	token, err := s.repo.GetByAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return 0, 0, err
	}
	if token.RevokedAt != nil {
		return 0, 0, domain.ErrAccessTokenExpired
	}
	if token.ExpiresAt.Before(time.Now()) {
		return 0, 0, domain.ErrAccessTokenExpired
	}
	return token.SessionID, token.UserID, nil
}

func (s *serviceStruct) GetByRefreshToken(ctx context.Context, refreshToken string) (int, int, error) {
	token, err := s.repo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return 0, 0, err
	}
	if token.RevokedAt != nil {
		return 0, 0, domain.ErrRefreshTokenExpired
	}
	return token.SessionID, token.UserID, nil
}

func (s *serviceStruct) CheckEmail(ctx context.Context, userID int) error {
	emailTable, err := s.repo.GetEmailTableByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return err
		}
	}
	if emailTable.UsedAt != nil {
		return domain.ErrEmailAlreadyVerified
	}
	return nil
}

func (s *serviceStruct) RevokeEmailTokens(ctx context.Context, userID int) error {
	return s.repo.RevokeEmailTokens(ctx, userID)
}

func (s *serviceStruct) SendEmail(ctx context.Context, email string, userID int, expiresAt int) error {
	serviceLogger := s.log.With("component", "service")
	token, err := s.GenerateToken()
	if err != nil {
		return err
	}
	hashedToken := hashToken(token)
	if err = s.repo.InsertEmailToken(
		ctx,
		userID,
		hashedToken,
		time.Now().Add(time.Duration(expiresAt)*time.Hour)); err != nil {
		return err
	}

	verifyURL := fmt.Sprintf("http://localhost:3000/verify-email?token=%s", token)

	params := &resend.SendEmailRequest{
		From:    "url-shortner test cuz I want to... <onboarding@resend.dev>",
		To:      []string{"delivered@resend.dev"},
		Subject: "Verify your email",
		Html: fmt.Sprintf(
			`<p>Click <a href="%s">here</a> to verify your email.</p>`,
			verifyURL,
		),
	}

	_, err = s.mail.Emails.SendWithContext(ctx, params)
	// this log is here cuz I dont have domain to send email from 😭
	serviceLogger.InfoContext(ctx, "failed to send email", "email", verifyURL)
	if err != nil {
		serviceLogger.ErrorContext(ctx, "failed to send email", "email", email, "error", err)
		return err
	}
	return nil
}

func (s *serviceStruct) VerifyEmail(ctx context.Context, token string) error {
	hashedToken := hashToken(token)
	emailTable, err := s.repo.GetEmailTableByToken(ctx, hashedToken)
	if errors.Is(err, domain.ErrTokenNotFound) {
		return domain.ErrEmailVerificationFailed
	}
	if err != nil {
		return err
	}
	if err = s.repo.MarkUserVerified(ctx, emailTable.UserID); err != nil {
		return err
	}
	if err = s.repo.RevokeEmailTokens(ctx, emailTable.UserID); err != nil {
		return err
	}
	return nil
}

func (s *serviceStruct) VerifyEmailToken(ctx context.Context, token string) (int, error) {
	hashedToken := hashToken(token)
	emailTable, err := s.repo.GetEmailTableByToken(ctx, hashedToken)
	if errors.Is(err, domain.ErrTokenNotFound) {
		return 0, domain.ErrEmailVerificationFailed
	}
	if err != nil {
		return 0, err
	}
	if err = s.repo.MarkUserVerified(ctx, emailTable.UserID); err != nil {
		return 0, err
	}
	if err = s.repo.RevokeEmailTokens(ctx, emailTable.UserID); err != nil {
		return 0, err
	}
	return emailTable.UserID, nil
}

func (s *serviceStruct) ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.ChangePasswordAndRevoke(ctx, userID, hashedPassword)
}

func (s *serviceStruct) SendForgotPasswordMail(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if _, err = s.repo.GetEmailTableByID(ctx, user.ID); err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if err = s.SendEmail(ctx, email, user.ID, 1); err != nil {
		return err
	}
	return nil
}

func (s *serviceStruct) GetLongURL(ctx context.Context, shortCode string) (string, error) {
	longURL, hit, err := s.repo.GetCachedLongURL(ctx, shortCode)
	if err != nil {
		return "", err
	}
	if hit {
		return longURL, nil
	}
	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	go func() {
		_ = s.repo.CacheShortURL(ctx, shortCode, url.LongURL)
	}()
	return url.LongURL, err
}

func (s *serviceStruct) URLClicked(ctx context.Context, shortCode string) error {
	return s.repo.URLClicked(ctx, shortCode)
}

func generateCode(n int) (string, error) {
	code := make([]byte, n)
	for i := range n {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		code[i] = alphabet[num.Int64()]
	}
	return string(code), nil
}

func (s *serviceStruct) InsertURL(ctx context.Context, longURL string, userID int) (string, error) {
	maxLength := 5
	shortCode, err := generateCode(maxLength)
	if err != nil {
		return "", err
	}
	if err = s.repo.InsertURL(ctx, shortCode, longURL, userID, time.Now()); err != nil {
		return "", err
	}
	if err = s.repo.CacheShortURL(ctx, shortCode, longURL); err != nil {
		return shortCode, domain.ErrCachingFailed
	}
	return shortCode, nil
}

func (s *serviceStruct) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error) {
	return s.repo.GetURLByUserID(ctx, userID)
}

func (s *serviceStruct) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	return s.repo.GetURLByShortCode(ctx, shortCode)
}

func (s *serviceStruct) DeleteURLByShortCode(ctx context.Context, shortCode string) error {
	return s.repo.DeleteURLByShortCode(ctx, shortCode)
}

func (s *serviceStruct) DeleteUser(ctx context.Context, userID int) error {
	return s.repo.DeleteUser(ctx, userID)
}
