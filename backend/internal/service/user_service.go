package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/backend/internal/domain"
)

type RepositoryInterface interface {
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
	InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	RevokeToken(ctx context.Context, sessionId int64) error
	GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error)
	GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error)
	ReplaceTokens(ctx context.Context, accessTokenHash []byte, refreshTokenHash []byte, sessionId int64, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
}

type ServiceInterface interface {
	Register(ctx context.Context, email string, name string, password string) (int, error)
	StoreTokens(ctx context.Context, userID int, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GenerateToken() (string, error)
	Login(ctx context.Context, email string, password string) (int, error)
	RevokeToken(ctx context.Context, refreshToken string) error
	ReplaceTokens(ctx context.Context, accessToken string, refreshToken string, userId int64, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetByRefreshToken(ctx context.Context, refreshToken string) (int64, error)
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
	return s.repo.RevokeToken(ctx, token.SessionId)
}

func (s *serviceStruct) ReplaceTokens(ctx context.Context, accessToken string, refreshToken string, userId int64, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	return s.repo.ReplaceTokens(ctx, hashToken(accessToken), hashToken(refreshToken), userId, accessExpiresAt, refreshExpiresAt)
}

func (s *serviceStruct) GetByAccessToken(ctx context.Context, accessToken string) (int64, int64, error) {
	token, err := s.repo.GetByAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return 0, 0, err
	}
	return token.SessionId, token.UserId, nil
}

func (s *serviceStruct) GetByRefreshToken(ctx context.Context, refreshToken string) (int64, error) {
	token, err := s.repo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return 0, err
	}
	if token.RevokedAt != nil {
		return 0, domain.ErrRefreshTokenExpired
	}
	return token.SessionId, nil
}
