package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zedmakesense/url-shortner/internal/app"
	"github.com/zedmakesense/url-shortner/pkg/logger"
)

const (
	shutdownTimeout = 5 * time.Second
)

func main() {
	log := logger.NewFromEnv()
	application, err := app.New()
	if err != nil {
		log.Error("failed to create application", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := application.Run(); err != nil {
			log.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("application started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		log.Error("failed to gracefully shutdown", "error", err)
	}
}
