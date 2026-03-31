package service

import (
	"context"
	"log/slog"
)

type RepositoryInterface interface {
	InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error)
}

type ServiceInterface interface {
	UserCreate(ctx context.Context, email string, name string, password string) (int, error)
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
