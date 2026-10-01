package inbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/notification/internal/inbox"
	"github.com/remodov/marketplace-system-go/services/notification/internal/migrations"
)

var (
	pool *pgxpool.Pool
	now  = time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://catalog:catalog@localhost:5440/notifications_test?sslmode=disable"
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", url)
	if err != nil {
		log.Fatal(err)
	}
	if err := migrations.Up(ctx, db); err != nil {
		log.Fatalf("тестовая база недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", err)
	}
	_ = db.Close()
	pool, err = pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func clearTables(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE notifications, processed_events"); err != nil {
		t.Fatal(err)
	}
}

func orderCreated(customer, seller uuid.UUID) []byte {
	return []byte(fmt.Sprintf(`{"orderId":"%s","customerId":"%s","sellerId":"%s","occurredAt":"2026-04-28T11:00:00Z","totalAmount":"4981.00","currency":"RUB","itemsCount":1}`,
		uuid.New(), customer, seller))
}

func TestProcess_sameEventTwice_createsOneNotification(t *testing.T) {
	clearTables(t)
	processor := inbox.NewProcessor(pool, fixedClock{})
	customer := uuid.New()
	event := inbox.IncomingEvent{ID: uuid.New(), Type: ordersv1.EventOrderCreated, Payload: orderCreated(customer, uuid.New())}

	first, err := processor.Process(context.Background(), event)
	if err != nil || !first {
		t.Fatalf("первая доставка должна обработаться: %v %v", first, err)
	}
	second, err := processor.Process(context.Background(), event)
	if err != nil || second {
		t.Fatalf("повторная доставка должна быть пропущена: %v %v", second, err)
	}

	items, err := processor.ListByUser(context.Background(), customer)
	if err != nil || len(items) != 1 || items[0].TemplateKey != "order-created" || items[0].Status != inbox.StatusPending {
		t.Fatalf("ожидали одно уведомление покупателю: %+v %v", items, err)
	}
}

func TestProcess_disputeOpened_goesToSeller(t *testing.T) {
	clearTables(t)
	processor := inbox.NewProcessor(pool, fixedClock{})
	customer, seller := uuid.New(), uuid.New()
	event := inbox.IncomingEvent{ID: uuid.New(), Type: ordersv1.EventDisputeOpened, Payload: orderCreated(customer, seller)}

	if _, err := processor.Process(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	toSeller, _ := processor.ListByUser(context.Background(), seller)
	toCustomer, _ := processor.ListByUser(context.Background(), customer)
	if len(toSeller) != 1 || len(toCustomer) != 0 {
		t.Fatalf("спор адресуется продавцу: продавцу %d, покупателю %d", len(toSeller), len(toCustomer))
	}
}

func TestProcess_payloadOffContract_isRejectedWithoutMarking(t *testing.T) {
	clearTables(t)
	processor := inbox.NewProcessor(pool, fixedClock{})
	event := inbox.IncomingEvent{ID: uuid.New(), Type: ordersv1.EventOrderCreated, Payload: []byte(`{"customerId":{"value":"abc"}}`)}

	_, err := processor.Process(context.Background(), event)

	if err == nil || errors.Is(err, inbox.ErrNoRecipient) {
		t.Fatalf("вложенный customerId это нарушение контракта, ожидали ошибку разбора: %v", err)
	}
	var count int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM processed_events").Scan(&count)
	if count != 0 {
		t.Fatalf("событие с плохим payload не должно помечаться обработанным")
	}
}

func TestProcess_withoutRecipient_isReportedNotStored(t *testing.T) {
	clearTables(t)
	processor := inbox.NewProcessor(pool, fixedClock{})
	event := inbox.IncomingEvent{ID: uuid.New(), Type: ordersv1.EventOrderCreated, Payload: []byte(`{"orderId":"` + uuid.NewString() + `"}`)}

	_, err := processor.Process(context.Background(), event)

	if !errors.Is(err, inbox.ErrNoRecipient) {
		t.Fatalf("ожидали ErrNoRecipient, получили %v", err)
	}
}
