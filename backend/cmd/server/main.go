package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/svdx9/talki/backend/internal/config"
	"github.com/svdx9/talki/backend/internal/server"
	"github.com/svdx9/talki/backend/internal/skills"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	//exhaustruct:ignore
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     cfg.LogLevel,
		AddSource: false,
	}))
	slog.SetDefault(logger)

	logger.Info("server starting",
		slog.Any("config", cfg.Redacted()),
	)

	skills, err := skills.NewMemoryRepositoryFromFile(skills.Catalog)
	if err != nil {
		logger.Error("skills load failed", "error", err)
		os.Exit(1)
	}
	srv, err := server.New(cfg, logger, skills)
	if err != nil {
		logger.Error("server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
