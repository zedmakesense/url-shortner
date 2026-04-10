package service

import (
	"context"
	"log/slog"

	"github.com/zedmakesense/url-shortner/internal/repository"
)

type PasswordService struct {
	repos *repository.Repositories
	log   *slog.Logger
}

func NewPasswordService(repos *repository.Repositories, log *slog.Logger) *PasswordService {
	return &PasswordService{
		repos: repos,
		log:   log,
	}
}

func (s *PasswordService) ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.repos.Password.ChangePasswordAndRevoke(ctx, userID, hashedPassword)
}
