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

	"github.com/remodov/marketplace-system-go/services/catalog/internal/bootstrap"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := bootstrap.FromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	app, err := bootstrap.Build(ctx, cfg, bootstrap.Deps{})
	if err != nil {
		return err
	}
	defer app.Close()

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: app.Handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("каталог слушает", "addr", cfg.HTTPAddr, "auth", cfg.AuthMode)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
