package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/skills"

	_ "golang.org/x/net/websocket"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     cfg.LogLevel,
		AddSource: false,
	}))
	slog.SetDefault(logger)

	logger.Info("server starting",
		slog.Any("config", cfg.Redacted()),
	)

	loadedSkills, err := skills.Load(skills.Catalog)
	if err != nil {
		logger.Error("skills load failed", "error", err)
		os.Exit(1)
	}
	skillIDs := make([]string, len(loadedSkills))
	for i, s := range loadedSkills {
		skillIDs[i] = s.ID
	}
	logger.Info("Loaded " + strconv.Itoa(len(loadedSkills)) + " skill(s): " + strings.Join(skillIDs, ", "))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := ":" + strconv.Itoa(cfg.Port)
	srv := &http.Server{Addr: addr}

	go func() {
		logger.Info("server listening on " + addr)
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
