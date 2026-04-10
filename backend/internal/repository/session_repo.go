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

type SessionRepository struct {
	db  *pgxpool.Pool
	log *slowQueryLogger
}

func NewSessionRepository(db *pgxpool.Pool, log *slog.Logger) *SessionRepository {
	return &SessionRepository{db: db, log: newSlowQueryLogger(log)}
}

func (r *SessionRepository) InsertSession(
	ctx context.Context,
	userID int,
	accessTokenHash []byte,
	refreshTokenHash []byte,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	start := time.Now()
	query := `
		INSERT INTO sessions (user_id, access_token_hash, refresh_token_hash, access_expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, $4, $5);
	`
	_, err := r.db.Exec(ctx, query, userID, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt)
	if err != nil {
		return err
	}

	r.log.Log(ctx, "InsertSession", start)
	return nil
}

func (r *SessionRepository) GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error) {
	start := time.Now()
	query := `
		SELECT session_id, user_id, access_token_hash, access_expires_at, revoked_at FROM sessions
		WHERE access_token_hash = $1
	`

	var token domain.Token

	err := r.db.QueryRow(
		ctx,
		query,
		accessToken).Scan(&token.SessionID, &token.UserID, &token.Token, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Token{}, domain.ErrTokenNotFound
		}
		return domain.Token{}, err
	}

	r.log.Log(ctx, "GetByAccessToken", start)
	return token, nil
}

func (r *SessionRepository) GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error) {
	start := time.Now()
	query := `
		SELECT session_id, user_id, refresh_token_hash, refresh_expires_at, revoked_at FROM sessions
		WHERE refresh_token_hash = $1;
	`

	var token domain.Token

	err := r.db.QueryRow(
		ctx,
		query,
		refreshToken).Scan(
		&token.SessionID,
		&token.UserID,
		&token.Token,
		&token.ExpiresAt,
		&token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Token{}, domain.ErrTokenNotFound
		}
		return domain.Token{}, err
	}

	r.log.Log(ctx, "GetByRefreshToken", start)
	return token, nil
}

func (r *SessionRepository) RevokeToken(ctx context.Context, sessionID int) error {
	start := time.Now()
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE session_id = $2;
	`
	cmdTag, err := r.db.Exec(ctx, query, time.Now(), sessionID)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTokenNotFound
	}

	r.log.Log(ctx, "RevokeToken", start)
	return nil
}

func (r *SessionRepository) RevokeTokens(ctx context.Context, userID int, sessionID int) error {
	start := time.Now()
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE user_id = $2
		AND session_id != $3
	`
	cmdTag, err := r.db.Exec(ctx, query, time.Now(), userID, sessionID)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTokenNotFound
	}

	r.log.Log(ctx, "RevokeTokens", start)
	return nil
}

func (r *SessionRepository) RevokeAllTokens(ctx context.Context, userID int, sessionID int) error {
	start := time.Now()
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE user_id=$2 AND session_id != $3;
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userID, sessionID)
	if err != nil {
		return err
	}

	r.log.Log(ctx, "RevokeAllTokens", start)
	return nil
}

func (r *SessionRepository) ReplaceTokens(
	ctx context.Context,
	accessTokenHash []byte,
	refreshTokenHash []byte,
	sessionID int,
	accessExpiresAt time.Time,
	refreshExpiresAt time.Time,
) error {
	start := time.Now()
	query := `
		UPDATE sessions
		SET access_token_hash = $1, refresh_token_hash = $2, access_expires_at = $3, refresh_expires_at = $4
	  WHERE session_id = $5;
  `

	_, err := r.db.Exec(ctx, query, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt, sessionID)
	if err != nil {
		return err
	}

	r.log.Log(ctx, "ReplaceTokens", start)
	return nil
}

func (r *SessionRepository) RevokeAllUserSessions(ctx context.Context, userID int) error {
	start := time.Now()
	query := `
		UPDATE sessions
		SET revoked_at = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserDoesNotExist
		}
		return err
	}

	r.log.Log(ctx, "RevokeAllUserSessions", start)
	return nil
}
