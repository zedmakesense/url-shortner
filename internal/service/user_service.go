package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"time"
)

type RepositoryInterface interface {
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
	InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
}

type ServiceInterface interface {
	UserCreate(ctx context.Context, email string, name string, password string) (int, error)
	StoreTokens(ctx context.Context, userID int, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) error
	GenerateToken() (string, error)
}

type serviceStruct struct {
	repo RepositoryInterface
	log  *slog.Logger
}

func NewService(repo RepositoryInterface, log *slog.Logger) *serviceStruct {
	return &serviceStruct{
		repo: repo,
		log:  log,
	}
}

func (s *serviceStruct) UserCreate(ctx context.Context, email string, name string, password string) (int, error) {
	svcLogger := s.log.With("component", "user_repository")
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
