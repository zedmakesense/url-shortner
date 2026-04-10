package service

import (
	"log/slog"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/repository"
)

type Services struct {
	User     *UserService
	Session  *SessionService
	Email    *EmailService
	URL      *URLService
	Password *PasswordService
}

func NewServices(repos *repository.Repositories, log *slog.Logger, mail *resend.Client) *Services {
	session := NewSessionService(repos, log)
	email := NewEmailService(repos, log, mail)
	url := NewURLService(repos, log)
	password := NewPasswordService(repos, log)
	user := NewUserService(repos, &Services{
		User:     nil, // avoid circular - user service is set below
		Session:  session,
		Email:    email,
		URL:      url,
		Password: password,
	}, log)
	// Fix circular reference
	user.services.User = user

	return &Services{
		User:     user,
		Session:  session,
		Email:    email,
		URL:      url,
		Password: password,
	}
}
