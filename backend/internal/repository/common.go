package repository

import (
	"context"
	"log/slog"
	"time"
)

const slowQueryThreshold = 100 * time.Millisecond

type slowQueryLogger struct {
	log *slog.Logger
}

func newSlowQueryLogger(log *slog.Logger) *slowQueryLogger {
	return &slowQueryLogger{log: log}
}

func (l *slowQueryLogger) Log(ctx context.Context, op string, start time.Time) {
	if elapsed := time.Since(start); elapsed > slowQueryThreshold {
		l.log.WarnContext(ctx, "slow query",
			"operation", op,
			"duration_ms", elapsed.Milliseconds(),
		)
	}
}

type baseRepo struct {
	db  DBQuerier
	log *slowQueryLogger
}

type DBQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (DBRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DBRowScanner
	Exec(ctx context.Context, sql string, args ...any) (DBResult, error)
}

type DBRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type DBRowScanner interface {
	Scan(dest ...any) error
}

type DBResult interface {
	RowsAffected() int64
}
