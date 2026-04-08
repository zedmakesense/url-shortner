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

func (r *repositoryStruct) logSlowQueries(ctx context.Context, op string, start time.Time) {
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		r.log.WarnContext(ctx, "slow query",
			"operation", op,
			"duration_ms", elapsed.Milliseconds(),
		)
	}
}

func (r *repositoryStruct) InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error) {
	start := time.Now()
	const query = `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING user_id;
	`
	var userID int

	err := r.db.QueryRow(ctx, query, email, name, hashedPassword).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, domain.ErrEmailAlreadyExists
		}
		return 0, err
	}
	r.logSlowQueries(ctx, "InsertUser", start)
	return userID, nil
}

func (r *repositoryStruct) InsertSession(ctx context.Context, userID int, accessTokenHash []byte, refreshTokenHash []byte, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	start := time.Now()
	query := `
		INSERT INTO sessions (user_id, access_token_hash, refresh_token_hash, access_expires_at, refresh_expires_at)
		VALUES ($1, $2, $3, $4, $5);
	`
	_, err := r.db.Exec(ctx, query, userID, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt)
	if err != nil {
		return err
	}

	r.logSlowQueries(ctx, "InsertSession", start)
	return nil
}

func (r *repositoryStruct) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	start := time.Now()
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
	r.logSlowQueries(ctx, "GetUserByEmail", start)
	return user, nil
}

func (r *repositoryStruct) GetUserByUserID(ctx context.Context, userID int) (domain.User, error) {
	start := time.Now()
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
	r.logSlowQueries(ctx, "GetUserByUserID", start)
	return user, nil
}

func (r *repositoryStruct) RevokeAllTokens(ctx context.Context, userId int, sessionId int) error {
	start := time.Now()
	query := `
		UPDATE sessions SET revoked_at = $1
		WHERE user_id=$2 AND session_id != $3;
	`
	_, err := r.db.Exec(ctx, query, time.Now(), userId, sessionId)

	if err != nil {
		return err
	}

	r.logSlowQueries(ctx, "RevokeAllTokens", start)
	return nil
}

func (r *repositoryStruct) RevokeToken(ctx context.Context, sessionId int) error {
	start := time.Now()
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

	r.logSlowQueries(ctx, "RevokeToken", start)
	return nil
}

func (r *repositoryStruct) GetByAccessToken(ctx context.Context, accessToken []byte) (domain.Token, error) {
	start := time.Now()
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

	r.logSlowQueries(ctx, "GetByAccessToken", start)
	return token, nil
}

func (r *repositoryStruct) GetByRefreshToken(ctx context.Context, refreshToken []byte) (domain.Token, error) {
	start := time.Now()
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

	r.logSlowQueries(ctx, "GetByRefreshToken", start)
	return token, nil
}

func (r *repositoryStruct) ReplaceTokens(ctx context.Context, accessTokenHash []byte, refreshTokenHash []byte, sessionId int, accessExpiresAt time.Time, refreshExpiresAt time.Time) error {
	start := time.Now()
	query := `
		UPDATE sessions
		SET access_token_hash = $1, refresh_token_hash = $2, access_expires_at = $3, refresh_expires_at = $4
	  WHERE session_id = $5;
  `

	_, err := r.db.Exec(ctx, query, accessTokenHash, refreshTokenHash, accessExpiresAt, refreshExpiresAt, sessionId)
	if err != nil {
		return err
	}

	r.logSlowQueries(ctx, "ReplaceTokens", start)
	return nil
}

func (r *repositoryStruct) GetEmailTableByID(ctx context.Context, userID int) (domain.EmailToken, error) {
	start := time.Now()
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

	r.logSlowQueries(ctx, "GetEmailTableByID", start)
	return token, nil
}

func (r *repositoryStruct) GetEmailTableByToken(ctx context.Context, hashedToken []byte) (domain.EmailToken, error) {
	start := time.Now()
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

	r.logSlowQueries(ctx, "GetEmailTableByToken", start)
	return token, nil
}

func (r *repositoryStruct) RevokeEmailTokens(ctx context.Context, userID int) error {
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

	r.logSlowQueries(ctx, "RevokeEmailTokens", start)
	return nil
}

func (r *repositoryStruct) InsertEmailToken(ctx context.Context, userID int, HashedToken []byte, expiresAt time.Time) error {
	start := time.Now()
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
	r.logSlowQueries(ctx, "InsertEmailToken", start)
	return nil
}

func (r *repositoryStruct) MarkUserVerified(ctx context.Context, userID int) error {
	start := time.Now()
	query := `
		UPDATE users
		SET is_email_verified = $1
		WHERE user_id = $2
	`
	_, err := r.db.Exec(ctx, query, true, userID)
	if err != nil {
		return err
	}

	r.logSlowQueries(ctx, "MarkUserVerified", start)
	return nil
}

func (r *repositoryStruct) ChangePasswordAndRevoke(ctx context.Context, userID int, hashedPassword string, sessionID int) error {
	start := time.Now()
	query1 := `
		UPDATE users
		SET password_hash = $1
		WHERE user_id = $2
	`
	query2 := `
		UPDATE sessions
		SET revoked_at = $1
		WHERE user_id = $2
		AND session_id != $3
	`
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, query1, hashedPassword, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, query2, time.Now(), userID, sessionID)
	if err != nil {
		return err
	}

	r.logSlowQueries(ctx, "ChangePasswordAndRevoke", start)
	return tx.Commit(ctx)
}

func (r *repositoryStruct) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	start := time.Now()
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
	r.logSlowQueries(ctx, "GetURLByShortCode", start)
	return url, nil
}

func (r *repositoryStruct) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error) {
	start := time.Now()
	query := `
		SELECT * FROM urls
		WHERE user_id = $1
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []domain.URL
	for rows.Next() {
		var u domain.URL
		if err := rows.Scan(
			&u.ID,
			&u.ShortCode,
			&u.LongURL,
			&u.UserID,
			&u.CreatedAt,
			&u.ExpiresAt,
			&u.ClickCount,
		); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	r.logSlowQueries(ctx, "GetURLByUserID", start)
	return urls, nil
}

func (r *repositoryStruct) InsertURL(ctx context.Context, shortCode string, longURL string, userID int, createdAt time.Time) error {
	start := time.Now()
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
	r.logSlowQueries(ctx, "InsertURL", start)
	return nil
}

func (r *repositoryStruct) URLClicked(ctx context.Context, shortCode string) error {
	start := time.Now()
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
	r.logSlowQueries(ctx, "URLClicked", start)
	return nil
}

func (r *repositoryStruct) DeleteURLByShortCode(ctx context.Context, shortCode string) error {
	start := time.Now()
	query := `
		DELETE FROM urls
		WHERE short_code = $1
	`

	_, err := r.db.Query(ctx, query, shortCode)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLDoesNotExist
		}
		return err
	}
	r.logSlowQueries(ctx, "DeleteURLByShortCode", start)
	return nil
}

func (r *repositoryStruct) DeleteUser(ctx context.Context, userID int) error {
	start := time.Now()
	query := `
		DELETE FROM users
		WHERE user_id = $1
	`
	_, err := r.db.Query(ctx, query, userID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	r.logSlowQueries(ctx, "DeleteUser", start)
	return nil
}
