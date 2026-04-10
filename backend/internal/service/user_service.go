package service

import (
	"context"
	"log/slog"

	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/repository"
)

type UserService struct {
	userRepo    *repository.UserRepository
	sessionSvc  *SessionService
	emailSvc    *EmailService
	passwordSvc *PasswordService
	log         *slog.Logger
}

func NewUserService(
	userRepo *repository.UserRepository,
	sessionSvc *SessionService,
	emailSvc *EmailService,
	passwordSvc *PasswordService,
	log *slog.Logger,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		sessionSvc:  sessionSvc,
		emailSvc:    emailSvc,
		passwordSvc: passwordSvc,
		log:         log,
	}
}

func (s *UserService) Register(ctx context.Context, email string, name string, password string) (int, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return 0, err
	}

	userID, err := s.userRepo.InsertUser(ctx, email, name, hashedPassword)
	if err != nil {
		return 0, err
	}

	if err = s.emailSvc.SendEmail(ctx, email, userID, 1); err != nil {
		return userID, err
	}
	return userID, nil
}

func (s *UserService) Login(ctx context.Context, email string, password string) (int, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return 0, err
	}
	return user.ID, comparePassword(user.HashedPassword, password)
}

func (s *UserService) GetUserByUserID(ctx context.Context, userID int) (domain.User, error) {
	return s.userRepo.GetUserByUserID(ctx, userID)
}

func (s *UserService) CheckPassword(ctx context.Context, userID int, password string) error {
	user, err := s.userRepo.GetUserByUserID(ctx, userID)
	if err != nil {
		return err
	}
	return comparePassword(user.HashedPassword, password)
}

func (s *UserService) DeleteUser(ctx context.Context, userID int) error {
	return s.userRepo.DeleteUser(ctx, userID)
}
