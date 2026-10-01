package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	httpadapter "github.com/remodov/marketplace-system-go/services/order/internal/adapter/in/http"
	kafkain "github.com/remodov/marketplace-system-go/services/order/internal/adapter/in/kafka"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
	kafkaadapter "github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/kafka"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/payment"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/persistence"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/persistence/migrations"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/system"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/query"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/usecase"
)

type App struct {
	Handler         http.Handler
	Pool            *pgxpool.Pool
	Relay           *usecase.OutboxRelay
	Expirer         *usecase.ExpireUnpaid
	Lifecycle       *usecase.LifecycleHandler
	PaymentConsumer *kafkain.PaymentConsumer
	publisher       out.ExternalEventPublisher
}

type Deps struct {
	Clock     out.Clock
	IDs       out.IDGenerator
	Auth      httpadapter.Authenticator
	Catalog   out.CatalogGateway
	Payment   out.PaymentGateway
	Publisher out.ExternalEventPublisher
}

const (
	outboxBatchSize = 100
	expireBatchSize = 200
)

func PaymentSettings(baseURL string) payment.Settings {
	return payment.Settings{BaseURL: baseURL, ConnectTimeout: 500 * time.Millisecond, RequestTimeout: 2 * time.Second}
}

func CatalogSettings(baseURL string) catalog.Settings {
	return catalog.Settings{
		BaseURL:            baseURL,
		ConnectTimeout:     500 * time.Millisecond,
		RequestTimeout:     time.Second,
		Attempts:           2,
		Backoff:            50 * time.Millisecond,
		BreakerMinRequests: 10,
		BreakerOpenFor:     60 * time.Second,
	}
}

func Migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Up(ctx, db)
}

func Build(ctx context.Context, cfg Config, deps Deps) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}
	if deps.Clock == nil {
		deps.Clock = system.Clock{}
	}
	if deps.IDs == nil {
		deps.IDs = system.IDGenerator{}
	}
	if deps.Catalog == nil {
		deps.Catalog = catalog.New(CatalogSettings(cfg.CatalogBaseURL))
	}
	if deps.Auth == nil {
		deps.Auth, err = authenticator(ctx, cfg)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}
	if deps.Publisher == nil {
		deps.Publisher = publisher(cfg)
	}
	if deps.Payment == nil {
		deps.Payment = payment.New(PaymentSettings(cfg.PaymentBaseURL))
	}

	orders := persistence.NewOrderRepository(pool)
	keys := persistence.NewIdempotencyKeys(pool)
	outbox := persistence.NewOutbox(pool, deps.IDs)
	processed := persistence.NewProcessedEvents(pool)
	uow := persistence.NewUnitOfWork(pool)

	create := usecase.NewCreateOrderHandler(orders, deps.Catalog, keys, outbox, deps.Clock, deps.IDs, uow)
	lifecycle := usecase.NewLifecycleHandler(orders, outbox, deps.Payment, processed, deps.Clock, uow)
	queries := query.NewHandler(orders)
	relay := usecase.NewOutboxRelay(outbox, deps.Publisher, deps.Clock, uow, outboxBatchSize)
	expirer := usecase.NewExpireUnpaid(orders, lifecycle, deps.Clock, cfg.ExpireAfter, expireBatchSize)

	app := &App{
		Handler:   httpadapter.NewRouter(deps.Auth, httpadapter.NewOrderHandler(create, lifecycle, queries), pool),
		Pool:      pool,
		Relay:     relay,
		Expirer:   expirer,
		Lifecycle: lifecycle,
		publisher: deps.Publisher,
	}
	if brokers := cfg.Brokers(); len(brokers) > 0 {
		app.PaymentConsumer = kafkain.NewPaymentConsumer(brokers, cfg.KafkaGroup, kafkain.NewPaymentHandler(lifecycle))
	}
	return app, nil
}

func publisher(cfg Config) out.ExternalEventPublisher {
	if brokers := cfg.Brokers(); len(brokers) > 0 {
		return kafkaadapter.NewPublisher(brokers, ordersv1.Topic)
	}
	return system.LogPublisher{}
}

func authenticator(ctx context.Context, cfg Config) (httpadapter.Authenticator, error) {
	if cfg.AuthMode == "jwt" {
		return httpadapter.NewJWTAuthenticator(ctx, cfg.JWKSURL, cfg.Issuer, cfg.Audience)
	}
	return httpadapter.LocalTokens{}, nil
}

func (a *App) Close() {
	if closer, ok := a.publisher.(io.Closer); ok {
		_ = closer.Close()
	}
	a.Pool.Close()
}
