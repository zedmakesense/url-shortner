package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/domain"
	"github.com/zedmakesense/url-shortner/internal/repository"
)

type EmailService struct {
	emailRepo *repository.EmailRepository
	userRepo  *repository.UserRepository
	log       *slog.Logger
	mail      *resend.Client
}

func NewEmailService(
	emailRepo *repository.EmailRepository,
	userRepo *repository.UserRepository,
	log *slog.Logger,
	mail *resend.Client,
) *EmailService {
	return &EmailService{
		emailRepo: emailRepo,
		userRepo:  userRepo,
		log:       log,
		mail:      mail,
	}
}

func (s *EmailService) SendEmail(ctx context.Context, email string, userID int, expiresAt int) error {
	serviceLogger := s.log.With("component", "service")
	token, err := GenerateRandomToken()
	if err != nil {
		return err
	}
	hashedToken := hashToken(token)
	if err = s.emailRepo.InsertEmailToken(
		ctx,
		userID,
		hashedToken,
		time.Now().Add(time.Duration(expiresAt)*time.Hour)); err != nil {
		return err
	}

	verifyURL := fmt.Sprintf("http://localhost:3000/verify-email?token=%s", token)

	params := &resend.SendEmailRequest{
		From:    "url-shortner test cuz I want to... <onboarding@resend.dev>",
		To:      []string{"delivered@resend.dev"},
		Subject: "Verify your email",
		Html: fmt.Sprintf(
			`<p>Click <a href="%s">here</a> to verify your email.</p>`,
			verifyURL,
		),
	}

	_, err = s.mail.Emails.SendWithContext(ctx, params)
	serviceLogger.InfoContext(ctx, "failed to send email", "email", verifyURL)
	if err != nil {
		serviceLogger.ErrorContext(ctx, "failed to send email", "email", email, "error", err)
		return err
	}
	return nil
}

func (s *EmailService) CheckEmail(ctx context.Context, userID int) error {
	emailTable, err := s.emailRepo.GetEmailTableByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return err
		}
	}
	if emailTable.UsedAt != nil {
		return domain.ErrEmailAlreadyVerified
	}
	return nil
}

func (s *EmailService) RevokeEmailTokens(ctx context.Context, userID int) error {
	return s.emailRepo.RevokeEmailTokens(ctx, userID)
}

func (s *EmailService) VerifyEmail(ctx context.Context, token string) error {
	hashedToken := hashToken(token)
	emailTable, err := s.emailRepo.GetEmailTableByToken(ctx, hashedToken)
	if errors.Is(err, domain.ErrTokenNotFound) {
		return domain.ErrEmailVerificationFailed
	}
	if err != nil {
		return err
	}
	if err = s.userRepo.MarkUserVerified(ctx, emailTable.UserID); err != nil {
		return err
	}
	if err = s.emailRepo.RevokeEmailTokens(ctx, emailTable.UserID); err != nil {
		return err
	}
	return nil
}

func (s *EmailService) VerifyEmailToken(ctx context.Context, token string) (int, error) {
	hashedToken := hashToken(token)
	emailTable, err := s.emailRepo.GetEmailTableByToken(ctx, hashedToken)
	if errors.Is(err, domain.ErrTokenNotFound) {
		return 0, domain.ErrEmailVerificationFailed
	}
	if err != nil {
		return 0, err
	}
	if err = s.userRepo.MarkUserVerified(ctx, emailTable.UserID); err != nil {
		return 0, err
	}
	if err = s.emailRepo.RevokeEmailTokens(ctx, emailTable.UserID); err != nil {
		return 0, err
	}
	return emailTable.UserID, nil
}

func (s *EmailService) SendForgotPasswordMail(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserDoesNotExist) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if _, err = s.emailRepo.GetEmailTableByID(ctx, user.ID); err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	if err = s.SendEmail(ctx, email, user.ID, 1); err != nil {
		return err
	}
	return nil
}

type EmailServiceInterface interface {
	SendEmail(ctx context.Context, email string, userID int, expiresAt int) error
	CheckEmail(ctx context.Context, userID int) error
	RevokeEmailTokens(ctx context.Context, userID int) error
	VerifyEmail(ctx context.Context, token string) error
	VerifyEmailToken(ctx context.Context, token string) (int, error)
	SendForgotPasswordMail(ctx context.Context, email string) error
}
