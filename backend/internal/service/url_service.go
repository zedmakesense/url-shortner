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
	urlRepo *repository.URLRepository
	log     *slog.Logger
}

func NewURLService(urlRepo *repository.URLRepository, log *slog.Logger) *URLService {
	return &URLService{
		urlRepo: urlRepo,
		log:     log,
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
	url, err := s.urlRepo.GetURLByShortCode(ctx, shortCode)
	return url.LongURL, err
}

func (s *URLService) URLClicked(ctx context.Context, shortCode string) error {
	return s.urlRepo.URLClicked(ctx, shortCode)
}

func (s *URLService) InsertURL(ctx context.Context, longURL string, userID int) (string, error) {
	maxLength := 5
	shortCode, err := generateCode(maxLength)
	if err != nil {
		return "", err
	}
	if err = s.urlRepo.InsertURL(ctx, shortCode, longURL, userID, time.Now()); err != nil {
		return "", err
	}
	return shortCode, nil
}

func (s *URLService) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error) {
	return s.urlRepo.GetURLByUserID(ctx, userID)
}

func (s *URLService) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	return s.urlRepo.GetURLByShortCode(ctx, shortCode)
}

func (s *URLService) DeleteURLByShortCode(ctx context.Context, shortCode string) error {
	return s.urlRepo.DeleteURLByShortCode(ctx, shortCode)
}

type URLServiceInterface interface {
	GetLongURL(ctx context.Context, shortCode string) (string, error)
	URLClicked(ctx context.Context, shortCode string) error
	InsertURL(ctx context.Context, longURL string, userID int) (string, error)
	GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error)
	GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error)
	DeleteURLByShortCode(ctx context.Context, shortCode string) error
}
