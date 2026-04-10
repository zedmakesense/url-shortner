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
