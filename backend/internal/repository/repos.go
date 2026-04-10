package repository

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct {
	User     *UserRepository
	Session  *SessionRepository
	Email    *EmailRepository
	URL      *URLRepository
	Password *PasswordRepository
}

func NewRepositories(db *pgxpool.Pool, log *slog.Logger) *Repositories {
	return &Repositories{
		User:     NewUserRepository(db, log),
		Session:  NewSessionRepository(db, log),
		Email:    NewEmailRepository(db, log),
		URL:      NewURLRepository(db, log),
		Password: NewPasswordRepository(db, log),
	}
}
