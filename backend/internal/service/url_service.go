package service

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math/big"
	"time"

	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/repository"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type URLService struct {
	repos *repository.Repositories
	log   *slog.Logger
}

func NewURLService(repos *repository.Repositories, log *slog.Logger) *URLService {
	return &URLService{
		repos: repos,
		log:   log,
	}
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

func (s *URLService) GetLongURL(ctx context.Context, shortCode string) (string, error) {
	url, err := s.repos.URL.GetURLByShortCode(ctx, shortCode)
	return url.LongURL, err
}

func (s *URLService) URLClicked(ctx context.Context, shortCode string) error {
	return s.repos.URL.URLClicked(ctx, shortCode)
}

func (s *URLService) InsertURL(ctx context.Context, longURL string, userID int) (string, error) {
	maxLength := 5
	shortCode, err := generateCode(maxLength)
	if err != nil {
		return "", err
	}
	if err = s.repos.URL.InsertURL(ctx, shortCode, longURL, userID, time.Now()); err != nil {
		return "", err
	}
	return shortCode, nil
}

func (s *URLService) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error) {
	return s.repos.URL.GetURLByUserID(ctx, userID)
}

func (s *URLService) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	return s.repos.URL.GetURLByShortCode(ctx, shortCode)
}

func (s *URLService) DeleteURLByShortCode(ctx context.Context, shortCode string) error {
	return s.repos.URL.DeleteURLByShortCode(ctx, shortCode)
}
