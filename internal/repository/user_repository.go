package repository

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type RepositoryStruct struct {
	db  *pgxpool.Pool
	rdb *redis.Client
	log *slog.Logger
}

func NewRepository(db *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *RepositoryStruct {
	return &RepositoryStruct{
		db:  db,
		rdb: rdb,
		log: log,
	}
}

func (r *RepositoryStruct) InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error) {
	repoLogger := r.log.With("component", "user_repository")
	const query = `
		INSERT INTO users (email, name, hashedPassword)
		VALUES ($1, $2, $3)
		RETURNING user_id
	`
	var user_ID int

	err := r.db.QueryRow(ctx, query, email, name, hashedPassword).Scan(&user_ID)
	if err != nil {
		repoLogger.ErrorContext(ctx, "user insertion failed", "email", email, "err", err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, domain.ErrEmailAlreadyExists
		}
		return 0, err
	}
	repoLogger.InfoContext(ctx, "user inserted", "user_ID", user_ID)
	return user_ID, nil
}
