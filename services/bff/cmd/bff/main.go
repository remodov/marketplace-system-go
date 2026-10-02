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

	"github.com/redis/go-redis/v9"

	"github.com/remodov/marketplace-system-go/services/bff/internal/config"
	"github.com/remodov/marketplace-system-go/services/bff/internal/httpapi"
	"github.com/remodov/marketplace-system-go/services/bff/internal/ratelimit"
	"github.com/remodov/marketplace-system-go/services/bff/internal/screen"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.FromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	limiter := ratelimit.NewLimiter(redis.NewClient(&redis.Options{Addr: cfg.RedisAddr}), cfg.RequestsPerMinute)
	screens := screen.NewAssembler(cfg.OrderURL, cfg.CatalogURL, cfg.PaymentURL)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(limiter, screens), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("bff слушает", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
