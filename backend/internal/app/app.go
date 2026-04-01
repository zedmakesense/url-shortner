package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/zedmakesense/url-shortner/backend/internal/config"
	"github.com/zedmakesense/url-shortner/backend/internal/logger"
	"github.com/zedmakesense/url-shortner/backend/internal/repository"
	"github.com/zedmakesense/url-shortner/backend/internal/service"
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
	log.Info("Application Started")

	dbconfig, err := pgxpool.ParseConfig(cfg.DatabaseUrl())
	if err != nil {
		log.Error("failed to parse Postgress config", "error", err)
		return nil, fmt.Errorf("Failed to parse Postgress config: %w", err)
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

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Error("failed to ping Postgress", "error", err)
		dbpool.Close()
		return nil, fmt.Errorf("failed to ping Postgress: %w", err)
	}

	log.Info("Postgress connection established")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Error("failed to ping Redis", "error", err)
		rdb.Close()
	}

	log.Info("Redis connection established")

	repositoryVariable := repository.NewRepository(dbpool, rdb, log)
	serviceVariable := service.NewService(repositoryVariable, log)
	router := NewRouter(serviceVariable, log)

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

func (a *App) Shutdown(ctx context.Context) error {
	a.log.Info("shutting down server")
	a.db.Close()
	a.redis.Close()
	return a.server.Shutdown(ctx)
}
