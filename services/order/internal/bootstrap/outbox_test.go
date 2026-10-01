package bootstrap_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
)

func TestOutbox_orderCreatedIsWrittenWithTheOrder(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "2490.50") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 2))

	expectStatus(t, rec, http.StatusCreated)
	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("в outbox %d строк, ожидали одну: %+v", len(rows), rows)
	}
	if rows[0].EventType != ordersv1.EventOrderCreated || rows[0].AggregateID.String() != decode(t, rec)["id"] || rows[0].Published {
		t.Fatalf("неожиданная строка outbox: %+v", rows[0])
	}
}

func TestOutbox_payloadFollowsExternalContract(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "2490.50") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, seller := uuid.New(), uuid.New()

	rec := postOrder(t, router, customerToken(customer), orderBody(uuid.New(), seller, 2))

	expectStatus(t, rec, http.StatusCreated)
	rows := outboxRows(t)
	if len(rows) != 1 {
		t.Fatalf("в outbox %d строк, ожидали одну", len(rows))
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(rows[0].Payload), &raw); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := []string{"currency", "customerId", "itemsCount", "occurredAt", "orderId", "sellerId", "totalAmount"}
	if !slices.Equal(keys, want) {
		t.Fatalf("поля payload %v, контракт ждёт %v", keys, want)
	}
	if raw["customerId"] != customer.String() || raw["sellerId"] != seller.String() {
		t.Fatalf("адресаты должны уезжать строками UUID: %s", rows[0].Payload)
	}
	if raw["totalAmount"] != "4981.00" || raw["currency"] != "RUB" || raw["itemsCount"] != 1.0 {
		t.Fatalf("сумма должна быть десятичной строкой: %s", rows[0].Payload)
	}
	var payload ordersv1.OrderCreatedPayload
	if err := json.Unmarshal([]byte(rows[0].Payload), &payload); err != nil || payload.OrderID.String() != decode(t, rec)["id"] {
		t.Fatalf("потребитель не прочитает payload в контрактный тип: %v %s", err, rows[0].Payload)
	}
}

func TestOutbox_nothingLeaksWhenTheTransactionRollsBack(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, key, body := uuid.New(), uuid.NewString(), orderBody(uuid.New(), uuid.New(), 1)

	expectStatus(t, postOrderWithKey(t, router, customerToken(customer), key, body), http.StatusCreated)
	expectStatus(t, postOrderWithKey(t, router, customerToken(customer), key, body), http.StatusOK)

	if rows := outboxRows(t); len(rows) != 1 {
		t.Fatalf("повтор не должен рождать второе событие: %+v", rows)
	}
}

func TestOutboxRelay_publishesAndMarksRows(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	publisher := &recordingPublisher{}
	app := newAppWith(t, catalog.New(testSettings(fake.URL)), publisher)
	first := givenOutboxRow(t, ordersv1.EventOrderCreated, `{"orderId":"a"}`)
	second := givenOutboxRow(t, ordersv1.EventOrderConfirmed, `{"orderId":"b"}`)

	published, err := app.Relay.Once(context.Background())
	if err != nil || published != 2 {
		t.Fatalf("ожидали две отправки, получили %d, %v", published, err)
	}
	again, err := app.Relay.Once(context.Background())
	if err != nil || again != 0 {
		t.Fatalf("второй круг не должен отправлять ничего: %d, %v", again, err)
	}

	messages := publisher.Published()
	if len(messages) != 2 || messages[0].ID != first || messages[1].ID != second {
		t.Fatalf("издатель получил %+v", messages)
	}
	if messages[0].EventType != ordersv1.EventOrderCreated || messages[0].AggregateType != "Order" || string(messages[0].Payload) != `{"orderId": "a"}` {
		t.Fatalf("сообщение собрано неверно: %+v", messages[0])
	}
	for _, row := range outboxRows(t) {
		if !row.Published {
			t.Fatalf("строка %s не помечена отправленной", row.ID)
		}
	}
}

func TestOutboxRelay_keepsRowWhenBrokerFails(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	publisher := &recordingPublisher{fail: errors.New("брокер лежит")}
	app := newAppWith(t, catalog.New(testSettings(fake.URL)), publisher)
	givenOutboxRow(t, ordersv1.EventOrderCreated, `{"orderId":"a"}`)

	published, err := app.Relay.Once(context.Background())

	if err == nil || published != 0 {
		t.Fatalf("при лежащем брокере relay должен вернуть ошибку: %d, %v", published, err)
	}
	rows := outboxRows(t)
	if len(rows) != 1 || rows[0].Published {
		t.Fatalf("строка должна остаться неотправленной: %+v", rows)
	}
}
