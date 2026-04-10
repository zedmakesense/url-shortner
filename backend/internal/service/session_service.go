package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/repository"
)

type SessionService struct {
	sessionRepo *repository.SessionRepository
	log         *slog.Logger
}

func NewSessionService(sessionRepo *repository.SessionRepository, log *slog.Logger) *SessionService {
	return &SessionService{
		sessionRepo: sessionRepo,
		log:         log,
	}
}

func (s *SessionService) StoreTokens(
	ctx context.Context,
	userID int,
	accessToken string,
	refreshToken string,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	return s.sessionRepo.InsertSession(
		ctx,
		userID,
		hashToken(accessToken),
		hashToken(refreshToken),
		accessExpiresAt,
		refreshExpiresAt)
}

func (s *SessionService) RevokeToken(ctx context.Context, refreshToken string) error {
	token, err := s.sessionRepo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return err
	}
	return s.sessionRepo.RevokeToken(ctx, token.SessionID)
}

func (s *SessionService) RevokeTokens(ctx context.Context, userID int, sessionID int) error {
	return s.sessionRepo.RevokeTokens(ctx, userID, sessionID)
}

func (s *SessionService) ReplaceTokens(
	ctx context.Context,
	accessToken string,
	refreshToken string,
	userID int,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	return s.sessionRepo.ReplaceTokens(
		ctx,
		hashToken(accessToken),
		hashToken(refreshToken),
		userID, accessExpiresAt, refreshExpiresAt)
}

func (s *SessionService) GetByAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	token, err := s.sessionRepo.GetByAccessToken(ctx, hashToken(accessToken))
	if err != nil {
		return 0, 0, err
	}
	return token.SessionID, token.UserID, nil
}

func (s *SessionService) GetByRefreshToken(ctx context.Context, refreshToken string) (int, int, error) {
	token, err := s.sessionRepo.GetByRefreshToken(ctx, hashToken(refreshToken))
	if err != nil {
		return 0, 0, err
	}
	if token.RevokedAt != nil {
		return 0, 0, domain.ErrRefreshTokenExpired
	}
	return token.SessionID, token.UserID, nil
}

func (s *SessionService) ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error) {
	token, err := s.sessionRepo.GetByAccessToken(ctx, hashToken(accessToken))
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

func (s *SessionService) GenerateToken() (string, error) {
	return GenerateRandomToken()
}

type SessionServiceInterface interface {
	StoreTokens(
		ctx context.Context,
		userID int,
		accessToken string,
		refreshToken string,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time,
	) error
	RevokeToken(ctx context.Context, refreshToken string) error
	RevokeTokens(ctx context.Context, userID int, sessionID int) error
	ReplaceTokens(
		ctx context.Context,
		accessToken string,
		refreshToken string,
		userID int,
		accessExpiresAt time.Time,
		refreshExpiresAt time.Time,
	) error
	GetByAccessToken(ctx context.Context, accessToken string) (int, int, error)
	GetByRefreshToken(ctx context.Context, refreshToken string) (int, int, error)
	ValidateAccessToken(ctx context.Context, accessToken string) (int, int, error)
	GenerateToken() (string, error)
}
