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
	"github.com/zedmakesense/url-shortner/backend/internal/domain"
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

func (r *repositoryStruct) GetUserByUserID(ctx context.Context, userID int) (domain.User, error) {
	query := `
		SELECT * FROM users
		WHERE user_id = $1;
	`
	var user domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Name, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserDoesNotExist
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *repositoryStruct) RevokeAllTokens(ctx context.Context, userId int, sessionId int) error {
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

func (r *repositoryStruct) RevokeToken(ctx context.Context, sessionId int) error {
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
	query := `
		SELECT session_id, user_id, access_token_hash, access_expires_at, revoked_at FROM sessions
		WHERE access_token_hash = $1
	`

	var token domain.Token

	err := r.db.QueryRow(ctx, query, accessToken).Scan(&token.SessionID, &token.UserID, &token.Token, &token.ExpiresAt, &token.RevokedAt)
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

	err := r.db.QueryRow(ctx, query, refreshToken).Scan(&token.SessionID, &token.UserID, &token.Token, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Token{}, domain.ErrTokenNotFound
		}
		return domain.Token{}, err
	}

	return token, nil
}

func (r *repositoryStruct) ReplaceTokens(ctx context.Context, accessTokenHash []byte, refreshTokenHash []byte, sessionId int, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	query := `
		UPDATE sessions
		SET access_token_hash = $1, refresh_token_hash = $2, access_expires_at = $3, refresh_expires_at = $4
	  WHERE session_id = $5;
  `

	_, err := r.db.Exec(ctx, query, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt, sessionId)
	if err != nil {
		return err
	}

	return nil
}

func (r *repositoryStruct) GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error) {
	query := `
		SELECT * FROM email_table
		WHERE user_id = $1
	`
	var token domain.EmailToken
	err := r.db.QueryRow(ctx, query, userID).Scan(&token.ID, &token.UserID, &token.HashedToken, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailToken{}, domain.ErrTokenNotFound
		}
		return domain.EmailToken{}, err
	}

	return token, nil
}

func (r *repositoryStruct) GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error) {
	query := `
		SELECT * FROM email_table
		WHERE token_hash = $1
		 	AND used_at IS NULL
	 		AND expires_at > NOW()
	`
	var token domain.EmailToken
	err := r.db.QueryRow(ctx, query, hashedToken).Scan(&token.ID, &token.UserID, &token.HashedToken, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailToken{}, domain.ErrTokenNotFound
		}
		return domain.EmailToken{}, err
	}

	return token, nil
}
func (r *repositoryStruct) RevokeEmailTokens(ctx context.Context, userID int) error {
	query := `
		UPDATE email_table
		SET used_at = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userID)
	if err != nil {
		return err
	}

	return nil
}

func (r *repositoryStruct) InsertEmailToken(ctx context.Context, userID int, HashedToken []byte, expiresAt time.Time) error {
	query := `
	INSERT INTO email_table (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	`
	_, err := r.db.Query(ctx, query, userID, HashedToken, expiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrTableAlreadyExists
		}
		return err
	}
	return nil
}

func (r *repositoryStruct) MarkUserVerified(ctx context.Context, userID int) error {
	query := `
		UPDATE users
		SET is_email_verified = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, true, userID)
	if err != nil {
		return err
	}

	return nil
}

func (r *repositoryStruct) ChangePassword(ctx context.Context, userID int, hashedPassword string) error {
	query := `
		UPDATE users
		SET password_hash = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, hashedPassword, userID)
	if err != nil {
		return err
	}

	return nil
}

func (r *repositoryStruct) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	query := `
		SELECT * FROM urls
		WHERE short_code = $1
	`
	var url domain.URL
	err := r.db.QueryRow(ctx, query, shortCode).Scan(&url.ID, &url.ShortCode, &url.LongURL, &url.UserID, &url.CreatedAt, &url.ExpiresAt, &url.ClickCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.URL{}, domain.ErrTokenNotFound
		}
		return domain.URL{}, err
	}
	return url, nil
}

func (r *repositoryStruct) InsertURL(ctx context.Context, shortCode string, longURL string, userID int, createdAt time.Time) error {
	query := `
		INSERT INTO urls (short_code, long_url, user_id, created_at)
		VALUES($1, $2, $3, $4)
	`
	_, err := r.db.Query(ctx, query, shortCode, longURL, userID, createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLAlreadyExist
		}
		return err
	}
	return nil
}

func (r *repositoryStruct) URLClicked(ctx context.Context, shortCode string) error {
	query := `
		UPDATE urls
		SET click_count = click_count + 1
		WHERE short_code = $1;
	`
	_, err := r.db.Query(ctx, query, shortCode)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLDoesNotExist
		}
		return err
	}
	return nil
}
