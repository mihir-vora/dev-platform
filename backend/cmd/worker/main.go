package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/dev-platform/backend/internal/config"
	"github.com/dev-platform/backend/internal/db"
	"github.com/dev-platform/backend/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := db.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	processor := worker.NewProcessor(cfg, store)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()

	processor.Run(ctx)
}
