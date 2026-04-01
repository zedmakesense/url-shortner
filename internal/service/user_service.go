package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/zedmakesense/url-shortner/internal/domain"
)

type RepositoryInterface interface {
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
	InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}

type ServiceInterface interface {
	Register(ctx context.Context, email string, name string, password string) (int, error)
	StoreTokens(ctx context.Context, userID int, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GenerateToken() (string, error)
	Login(ctx context.Context, email string, password string) (int, error)
}

type serviceStruct struct {
	repo RepositoryInterface
	log  *slog.Logger
}

func NewService(repo RepositoryInterface, log *slog.Logger) ServiceInterface {
	return &serviceStruct{
		repo: repo,
		log:  log,
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
