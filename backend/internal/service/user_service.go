package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/backend/internal/domain"
)

type RepositoryInterface interface {
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
	InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	RevokeToken(ctx context.Context, sessionId int) error
	GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error)
	GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error)
	ReplaceTokens(ctx context.Context, accessTokenHash []byte, refreshTokenHash []byte, sessionId int, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error)
	GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error)
	RevokeEmailTokens(ctx context.Context, userID int) error
	InsertEmailToken(ctx context.Context, userID int, HashedToken []byte, expiresAt time.Time) error
	MarkUserVerified(ctx context.Context, userID int) error
	ChangePassword(ctx context.Context, userID int, hashedPassword string) error
}

type ServiceInterface interface {
	Register(ctx context.Context, email string, name string, password string) (int, error)
	StoreTokens(ctx context.Context, userID int, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GenerateToken() (string, error)
	Login(ctx context.Context, email string, password string) (int, error)
	RevokeToken(ctx context.Context, refreshToken string) error
	ReplaceTokens(ctx context.Context, accessToken string, refreshToken string, userId int, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetByRefreshToken(ctx context.Context, refreshToken string) (int, error)
	CheckEmail(ctx context.Context, email string, userID int) error
	RevokeEmailTokens(ctx context.Context, userID int) error
	SendEmail(ctx context.Context, email string, userID int, expiresAt int) error
	VerifyEmail(ctx context.Context, token string) error
	SendForgotPasswordMail(ctx context.Context, email string) error
}

type serviceStruct struct {
	repo RepositoryInterface
	log  *slog.Logger
	mail *resend.Client
}

func NewService(repo RepositoryInterface, log *slog.Logger, mail *resend.Client) ServiceInterface {
	return &serviceStruct{
		repo: repo,
		log:  log,
		mail: mail,
	}
}

func (s *serviceStruct) Register(ctx context.Context, email string, name string, password string) (int, error) {
	svcLogger := s.log.With("component", "user_service")
	hashedPassword, err := hashPassword(password)
	if err != nil {
		svcLogger.ErrorContext(ctx, "password hashing function failed", "error", err)
		return 0, err
	}

	userID, err := s.repo.InsertUser(ctx, email, name, hashedPassword)
	if err != nil {
		return 0, err
	}

	svcLogger.InfoContext(ctx, "user created", "user_id", userID, "email", email)
	if err := s.SendEmail(ctx, email, userID); err != nil {
		return userID, err
	}
	return userID, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (s *serviceStruct) GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func (s *serviceStruct) StoreTokens(ctx context.Context, userID int, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	return s.repo.InsertSession(ctx, userID, hashToken(accessToken), hashToken(refreshToken), accessExpiresAt, refreshExpiresAt)
}

func (s *serviceStruct) Login(ctx context.Context, email string, password string) (int, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return 0, err
	}
	return user.ID, comparePassword(user.HashedPassword, password)
}

func (s *serviceStruct) RevokeToken(ctx context.Context, refreshToken string) error {
	token, err := s.repo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return err
	}
	return s.repo.RevokeToken(ctx, token.SessionID)
}

func (s *serviceStruct) ReplaceTokens(ctx context.Context, accessToken string, refreshToken string, userId int, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	return s.repo.ReplaceTokens(ctx, hashToken(accessToken), hashToken(refreshToken), userId, accessExpiresAt, refreshExpiresAt)
}

func (s *serviceStruct) GetByAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	token, err := s.repo.GetByAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return 0, 0, err
	}
	return token.SessionID, token.UserID, nil
}

func (s *serviceStruct) GetByRefreshToken(ctx context.Context, refreshToken string) (int, error) {
	token, err := s.repo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return 0, err
	}
	if token.RevokedAt != nil {
		return 0, domain.ErrRefreshTokenExpired
	}
	return token.SessionID, nil
}

func (s *serviceStruct) CheckEmail(ctx context.Context, email string, userID int) error {
	emailTable, err := s.repo.GetEmailTableByID(ctx, userID)
	if err != nil {
		if err != domain.ErrTokenNotFound {
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
	token, err := s.GenerateToken()
	if err != nil {
		return err
	}
	hashedToken := hashToken(token)
	s.repo.InsertEmailToken(ctx, userID, hashedToken, time.Now().Add(time.Duration(expiresAt)*time.Hour))

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
	fmt.Println(verifyURL)
	if err != nil {
		return err
	}
	return nil
}

func (s *serviceStruct) VerifyEmail(ctx context.Context, token string) error {
	hashedToken := hashToken(token)
	emailTable, err := s.repo.GetEmailTableByToken(ctx, hashedToken)
	if err == domain.ErrTokenNotFound {
		return domain.ErrEmailVerificationFailed
	}
	if err != nil {
		return err
	}
	if err := s.repo.MarkUserVerified(ctx, emailTable.UserID); err != nil {
		return err
	}
	if err := s.repo.RevokeEmailTokens(ctx, emailTable.UserID); err != nil {
		return err
	}
	return nil
}

func (s *serviceStruct) ChangePassword(ctx context.Context, userID int, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.ChangePassword(ctx, userID, hashedPassword)
}

func (s *serviceStruct) SendForgotPasswordMail(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if _, err := s.repo.GetEmailTableByID(ctx, user.ID); err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if err != nil {
		return err
	}
	if err := s.SendEmail(ctx, email, user.ID, 1); err != nil {
		return err
	}
	return nil
}
