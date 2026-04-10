package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zedmakesense/url-shortner/internal/domain"
)

type PasswordRepository struct {
	db  *pgxpool.Pool
	log *slowQueryLogger
}

func NewPasswordRepository(db *pgxpool.Pool, log *slog.Logger) *PasswordRepository {
	return &PasswordRepository{db: db, log: newSlowQueryLogger(log)}
}

func (r *PasswordRepository) ChangePasswordAndRevoke(
	ctx context.Context,
	userID int,
	hashedPassword string,
) error {
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
	`
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(ctx, query1, hashedPassword, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, query2, time.Now(), userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserDoesNotExist
		}
		return err
	}

	r.log.Log(ctx, "ChangePasswordAndRevoke", start)
	return tx.Commit(ctx)
}
