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

type URLRepository struct {
	db  *pgxpool.Pool
	log *slowQueryLogger
}

func NewURLRepository(db *pgxpool.Pool, log *slog.Logger) *URLRepository {
	return &URLRepository{db: db, log: newSlowQueryLogger(log)}
}

func (r *URLRepository) GetURLByShortCode(ctx context.Context, shortCode string) (domain.URL, error) {
	start := time.Now()
	query := `
		SELECT id, short_code, long_url, user_id, created_at, expires_at, click_count FROM urls
		WHERE short_code = $1
	`
	var url domain.URL
	err := r.db.QueryRow(
		ctx,
		query,
		shortCode).Scan(
		&url.ID,
		&url.ShortCode,
		&url.LongURL,
		&url.UserID,
		&url.CreatedAt,
		&url.ExpiresAt,
		&url.ClickCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.URL{}, domain.ErrTokenNotFound
		}
		return domain.URL{}, err
	}
	r.log.Log(ctx, "GetURLByShortCode", start)
	return url, nil
}

func (r *URLRepository) GetURLByUserID(ctx context.Context, userID int) ([]domain.URL, error) {
	start := time.Now()
	query := `
		SELECT id, short_code, long_url, user_id, created_at, expires_at, click_count FROM urls
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
		if err = rows.Scan(
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
	if err = rows.Err(); err != nil {
		return nil, err
	}

	r.log.Log(ctx, "GetURLByUserID", start)
	return urls, nil
}

func (r *URLRepository) InsertURL(
	ctx context.Context,
	shortCode string,
	longURL string,
	userID int,
	createdAt time.Time,
) error {
	start := time.Now()
	query := `
		INSERT INTO urls (short_code, long_url, user_id, created_at)
		VALUES($1, $2, $3, $4)
	`
	rows, err := r.db.Query(ctx, query, shortCode, longURL, userID, createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLAlreadyExist
		}
		return err
	}
	defer rows.Close()
	r.log.Log(ctx, "InsertURL", start)
	return nil
}

func (r *URLRepository) URLClicked(ctx context.Context, shortCode string) error {
	start := time.Now()
	query := `
		UPDATE urls
		SET click_count = click_count + 1
		WHERE short_code = $1;
	`
	rows, err := r.db.Query(ctx, query, shortCode)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLDoesNotExist
		}
		return err
	}
	defer rows.Close()
	r.log.Log(ctx, "URLClicked", start)
	return nil
}

func (r *URLRepository) DeleteURLByShortCode(ctx context.Context, shortCode string) error {
	start := time.Now()
	query := `
		DELETE FROM urls
		WHERE short_code = $1
	`

	rows, err := r.db.Query(ctx, query, shortCode)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLDoesNotExist
		}
		return err
	}
	defer rows.Close()
	r.log.Log(ctx, "DeleteURLByShortCode", start)
	return nil
}
