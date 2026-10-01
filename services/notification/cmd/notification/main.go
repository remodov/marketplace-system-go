package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/remodov/marketplace-system-go/services/notification/internal/config"
	"github.com/remodov/marketplace-system-go/services/notification/internal/consumer"
	"github.com/remodov/marketplace-system-go/services/notification/internal/httpapi"
	"github.com/remodov/marketplace-system-go/services/notification/internal/inbox"
	"github.com/remodov/marketplace-system-go/services/notification/internal/migrations"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

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

	if err := migrate(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	processor := inbox.NewProcessor(pool, systemClock{})
	var background sync.WaitGroup
	background.Go(func() {
		if err := consumer.New(cfg.KafkaBrokers, cfg.KafkaGroup, cfg.KafkaTopic, processor).Run(ctx); err != nil {
			slog.Error("консьюмер остановлен", "err", err)
		}
	})

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(processor, pool, cfg.AdminToken), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("сервис уведомлений слушает", "addr", cfg.HTTPAddr, "kafka", cfg.KafkaBrokers, "group", cfg.KafkaGroup)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	background.Wait()
	return nil
}

func migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Up(ctx, db)
}
