package service

import (
	"context"
	"log/slog"

	"github.com/zedmakesense/url-shortner/internal/repository"
)

type PasswordService struct {
	passwordRepo *repository.PasswordRepository
	log          *slog.Logger
}

func NewPasswordService(passwordRepo *repository.PasswordRepository, log *slog.Logger) *PasswordService {
	return &PasswordService{
		passwordRepo: passwordRepo,
		log:          log,
	}
}

func (s *PasswordService) ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.passwordRepo.ChangePasswordAndRevoke(ctx, userID, hashedPassword)
}

type PasswordServiceInterface interface {
	ChangePasswordAndRevoke(ctx context.Context, userID int, password string) error
}
