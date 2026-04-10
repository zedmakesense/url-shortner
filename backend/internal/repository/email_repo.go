package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type EmailRepository struct {
	db  *pgxpool.Pool
	log *slowQueryLogger
}

func NewEmailRepository(db *pgxpool.Pool, log *slog.Logger) *EmailRepository {
	return &EmailRepository{db: db, log: newSlowQueryLogger(log)}
}

func (r *EmailRepository) GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error) {
	start := time.Now()
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at FROM email_table
		WHERE user_id = $1
	`
	var token domain.EmailToken
	err := r.db.QueryRow(
		ctx,
		query,
		userID).Scan(
		&token.ID,
		&token.UserID,
		&token.HashedToken,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailToken{}, domain.ErrTokenNotFound
		}
		return domain.EmailToken{}, err
	}

	r.log.Log(ctx, "GetEmailTableByID", start)
	return token, nil
}

func (r *EmailRepository) GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error) {
	start := time.Now()
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at FROM email_table
		WHERE token_hash = $1
		 	AND used_at IS NULL
	 		AND expires_at > NOW()
	`
	var token domain.EmailToken
	err := r.db.QueryRow(
		ctx,
		query,
		hashedToken).Scan(
		&token.ID,
		&token.UserID,
		&token.HashedToken,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailToken{}, domain.ErrTokenNotFound
		}
		return domain.EmailToken{}, err
	}

	r.log.Log(ctx, "GetEmailTableByToken", start)
	return token, nil
}

func (r *EmailRepository) RevokeEmailTokens(ctx context.Context, userID int) error {
	start := time.Now()
	query := `
		UPDATE email_table
		SET used_at = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userID)
	if err != nil {
		return err
	}

	r.log.Log(ctx, "RevokeEmailTokens", start)
	return nil
}

func (r *EmailRepository) InsertEmailToken(
	ctx context.Context,
	userID int,
	hashedToken []byte,
	expiresAt time.Time,
) error {
	start := time.Now()
	query := `
	INSERT INTO email_table (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	`
	rows, err := r.db.Query(ctx, query, userID, hashedToken, expiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	defer rows.Close()
	r.log.Log(ctx, "InsertEmailToken", start)
	return nil
}
