package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type repositoryStruct struct {
	db  *pgxpool.Pool
	rdb *redis.Client
	log *slog.Logger
}

func NewRepository(db *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *repositoryStruct {
	return &repositoryStruct{
		db:  db,
		rdb: rdb,
		log: log,
	}
}

func (r *repositoryStruct) InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error) {
	repoLogger := r.log.With("component", "user_repository")
	const query = `
		INSERT INTO users (email, name, hashedPassword)
		VALUES ($1, $2, $3)
		RETURNING user_id;
	`
	var userID int

	err := r.db.QueryRow(ctx, query, email, name, hashedPassword).Scan(&userID)
	if err != nil {
		repoLogger.ErrorContext(ctx, "user insertion failed", "email", email, "err", err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, domain.ErrEmailAlreadyExists
		}
		repoLogger.ErrorContext(ctx, "user instertion failed", "userID", userID)
		return 0, err
	}
	repoLogger.InfoContext(ctx, "user inserted", "userID", userID)
	return userID, nil
}

func (r *repositoryStruct) InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	repoLogger := r.log.With("component", "user_repository")
	query := `
		INSERT INTO sessions (user_id, access_token_hash, refresh_token_hash, access_expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, $4, $5);
	`
	_, err := r.db.Exec(ctx, query, userID, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt)
	if err != nil {
		repoLogger.ErrorContext(ctx, "session instertion failed", "userID", userID)
		return err
	}

	repoLogger.InfoContext(ctx, "session inserted", "userID", userID)
	return nil
}

func (r *repositoryStruct) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	query := `
		SELECT * FROM users
		WHERE email=$1;
	`
	var user domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.Name, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserDoesNotExist
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *repositoryStruct) RevokeAllTokens(ctx context.Context, userId int64, sessionId int64) error {
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE user_id=$2 AND session_id != $3;
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userId, sessionId)

	if err != nil {
		return err
	}

	return nil
}

func (r *repositoryStruct) RevokeToken(ctx context.Context, sessionId int64) error {
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE session_id=$2;
	`
	cmdTag, err := r.db.Exec(ctx, query, time.Now(), sessionId)

	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTokenNotFound
	}

	return nil
}

func (r *repositoryStruct) GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error) {
	query := "SELECT session_id, user_id, access_token_hash, access_expires_at, revoked_at FROM sessions WHERE access_token_hash = $1;"

	var token domain.Token

	err := r.db.QueryRow(ctx, query, accessToken).Scan(&token.SessionId, &token.UserId, &token.Token, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Token{}, domain.ErrTokenNotFound
		}
		return domain.Token{}, err
	}

	return token, nil
}
func (r *repositoryStruct) GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error) {
	query := `
		SELECT session_id, user_id, refresh_token_hash, refresh_expires_at, revoked_at FROM sessions
		WHERE refresh_token_hash = $1;
	`

	var token domain.Token

	err := r.db.QueryRow(ctx, query, refreshToken).Scan(&token.SessionId, &token.UserId, &token.Token, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Token{}, domain.ErrTokenNotFound
		}
		return domain.Token{}, err
	}

	return token, nil
}

func (r *repositoryStruct) ReplaceTokens(ctx context.Context, accessTokenHash []byte, refreshTokenHash []byte, sessionId int64, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	query := `
		UPDATE sessions SET access_token_hash = $1, refresh_token_hash = $2, access_expires_at = $3, refresh_expires_at = $4
	  WHERE session_id = $5;
  `

	_, err := r.db.Exec(ctx, query, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt, sessionId)
	if err != nil {
		return err
	}

	return nil
}
