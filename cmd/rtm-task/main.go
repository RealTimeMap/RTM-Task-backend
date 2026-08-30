package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"RTM-Task/internal/app"
	"RTM-Task/internal/config"
	"RTM-Task/internal/infrastructure/persistence/postgres"
	transporthttp "RTM-Task/internal/transport/http"
	"RTM-Task/internal/utils/database"
	"RTM-Task/internal/utils/logger"
)

func main() {
	cfg := config.MustLoad()

	log := logger.New(cfg.Env)
	defer func() { _ = log.Sync() }()

	db := database.MustNew(cfg.Database, log)
	defer func() {
		if err := database.Close(db); err != nil {
			log.Error("close database failed", zap.Error(err))
		}
	}()

	if err := postgres.AutoMigrate(db); err != nil {
		log.Fatal("migrate database failed", zap.Error(err))
	}

	container := app.MustContainer(cfg, db, log)
	server := transporthttp.NewServer(container)

	// Сервер слушает в отдельной горутине, чтобы main мог ждать сигнал.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatal("http server failed", zap.Error(err))
		}
	case sig := <-stop:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown failed", zap.Error(err))
		}
		container.Socket.Close()
	}

	log.Info("service stopped")
}
