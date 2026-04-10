package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/resend/resend-go/v3"
	"github.com/zedmakesense/url-shortner/internal/config"
	"github.com/zedmakesense/url-shortner/internal/logger"
	"github.com/zedmakesense/url-shortner/internal/repository"
	"github.com/zedmakesense/url-shortner/internal/service"
)

type App struct {
	cfg    *config.Config
	server *http.Server
	redis  *redis.Client
	db     *pgxpool.Pool
	log    *slog.Logger
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log := logger.NewLogger(cfg.Log)

	mail := resend.NewClient(cfg.Resend.APIKey)

	dbconfig, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		log.Error("failed to parse Postgress config", "error", err)
		return nil, fmt.Errorf("failed to parse Postgress config: %w", err)
	}
	if cfg.DB.MaxOpenConns > math.MaxInt32 || cfg.DB.MaxOpenConns < 0 {
		log.Error("max open connections value out of range for int32", "error", err)
		return nil, fmt.Errorf("max open connections value %d out of range for int32", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns > math.MaxInt32 || cfg.DB.MaxIdleConns < 0 {
		log.Error("max idle connections value out of range for int32", "error", err)
		return nil, fmt.Errorf("max idle connections value %d out of range for int32", cfg.DB.MaxIdleConns)
	}

	dbconfig.MaxConns = int32(cfg.DB.MaxOpenConns)
	dbconfig.MinConns = int32(cfg.DB.MaxIdleConns)
	dbconfig.MaxConnLifetime = cfg.DB.ConnMaxLifetime
	dbconfig.MaxConnIdleTime = cfg.DB.ConnMaxIdleTime

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbconfig)
	if err != nil {
		log.Error("failed to connect to DB", "error", err)
		return nil, fmt.Errorf("failed to connect to Postgress: %w", err)
	}

	if err = dbpool.Ping(context.Background()); err != nil {
		log.Error("failed to ping Postgress", "error", err)
		dbpool.Close()
		return nil, fmt.Errorf("failed to ping Postgress: %w", err)
	}

	log.Info("postgress connection established")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err = rdb.Ping(context.Background()).Err(); err != nil {
		log.Error("failed to ping Redis", "error", err)
		if err = rdb.Close(); err != nil {
			log.Error("rdb.Close", "error", err)
		}
	}

	log.Info("redis connection established")

	repos := repository.NewRepositories(dbpool, log)
	services := service.NewServices(repos, log, mail)

	router := NewRouter(services, log)

	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
		IdleTimeout:  cfg.App.IdleTimeout,
	}
	return &App{
		cfg:    cfg,
		server: server,
		redis:  rdb,
		db:     dbpool,
		log:    log,
	}, nil
}

func (a *App) Run() error {
	a.log.Info("HTTP server listening", "port", a.cfg.App.Port)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context, log *slog.Logger) error {
	a.log.InfoContext(ctx, "shutting down server")
	a.db.Close()
	if err := a.redis.Close(); err != nil {
		log.ErrorContext(ctx, "rdb.Close", "error", err)
	}
	return a.server.Shutdown(ctx)
}
