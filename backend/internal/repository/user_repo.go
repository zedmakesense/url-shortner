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

type UserRepository struct {
	db  *pgxpool.Pool
	log *slowQueryLogger
}

func NewUserRepository(db *pgxpool.Pool, log *slog.Logger) *UserRepository {
	return &UserRepository{db: db, log: newSlowQueryLogger(log)}
}

func (r *UserRepository) InsertUser(ctx context.Context, email string, name string, hashedPassword string) (int, error) {
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
	r.log.Log(ctx, "InsertUser", start)
	return userID, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	start := time.Now()
	query := `
		SELECT user_id, email, name, password_hash, is_email_verified, created_at FROM users
		WHERE email=$1;
	`
	var user domain.User
	err := r.db.QueryRow(
		ctx,
		query,
		email).Scan(&user.ID, &user.Name, &user.Email, &user.HashedPassword, &user.IsEmailVerified, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserDoesNotExist
		}
		return domain.User{}, err
	}
	r.log.Log(ctx, "GetUserByEmail", start)
	return user, nil
}

func (r *UserRepository) GetUserByUserID(ctx context.Context, userID int) (domain.User, error) {
	start := time.Now()
	query := `
		SELECT user_id, email, name, password_hash, is_email_verified, created_at FROM users
		WHERE user_id = $1;
	`
	var user domain.User
	err := r.db.QueryRow(
		ctx,
		query,
		userID).Scan(&user.ID, &user.Name, &user.Email, &user.HashedPassword, &user.IsEmailVerified, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserDoesNotExist
		}
		return domain.User{}, err
	}
	r.log.Log(ctx, "GetUserByUserID", start)
	return user, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
	start := time.Now()
	query := `
		DELETE FROM users
		WHERE user_id = $1
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserDoesNotExist
		}
		return err
	}
	defer rows.Close()
	r.log.Log(ctx, "DeleteUser", start)
	return nil
}

func (r *UserRepository) MarkUserVerified(ctx context.Context, userID int) error {
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

	r.log.Log(ctx, "MarkUserVerified", start)
	return nil
}
