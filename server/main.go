package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/router"
)

func main() {
	cfg := config.Load()

	logger.Init(cfg.Log.Level, cfg.Log.Mode)
	defer func() { _ = logger.Sync() }()

	if cfg.Database.Driver != "" {
		if err := database.Init(cfg.Database); err != nil {
			logger.L().Fatalw("database init failed", "err", err)
		}
	}

	engine := router.New(cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.L().Infow("server started", "port", cfg.Server.Port, "mode", cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.L().Errorw("server shutdown failed", "err", err)
	}
	logger.L().Info("server exited")
}
