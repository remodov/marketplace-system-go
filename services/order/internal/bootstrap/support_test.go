package bootstrap_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
	"github.com/remodov/marketplace-system-go/services/order/internal/bootstrap"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

var (
	pool        *pgxpool.Pool
	databaseURL string
	now         = time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

func TestMain(m *testing.M) {
	databaseURL = os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://catalog:catalog@localhost:5440/orders_test?sslmode=disable"
	}
	ctx := context.Background()
	if err := bootstrap.Migrate(ctx, databaseURL); err != nil {
		log.Fatalf("тестовая база недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", err)
	}
	var err error
	pool, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func newApp(t *testing.T, gateway out.CatalogGateway) http.Handler {
	t.Helper()
	return newAppWith(t, gateway, &recordingPublisher{}).Handler
}

func newAppWith(t *testing.T, gateway out.CatalogGateway, publisher out.ExternalEventPublisher) *bootstrap.App {
	t.Helper()
	app, err := bootstrap.Build(context.Background(), bootstrap.Config{DatabaseURL: databaseURL, AuthMode: "local"},
		bootstrap.Deps{Clock: fixedClock{}, Catalog: gateway, Publisher: publisher})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	return app
}

type recordingPublisher struct {
	mu       sync.Mutex
	messages []out.OutboxMessage
	fail     error
}

func (p *recordingPublisher) Publish(_ context.Context, m out.OutboxMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail != nil {
		return p.fail
	}
	p.messages = append(p.messages, m)
	return nil
}

func (p *recordingPublisher) Published() []out.OutboxMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]out.OutboxMessage(nil), p.messages...)
}

type outboxRow struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   string
	Payload     string
	Published   bool
}

func outboxRows(t *testing.T) []outboxRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT id, aggregate_id, event_type, payload::text, published_at IS NOT NULL FROM outbox ORDER BY occurred_at, id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []outboxRow
	for rows.Next() {
		var r outboxRow
		if err := rows.Scan(&r.ID, &r.AggregateID, &r.EventType, &r.Payload, &r.Published); err != nil {
			t.Fatal(err)
		}
		result = append(result, r)
	}
	return result
}

func givenOutboxRow(t *testing.T, eventType, payload string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbox (id, aggregate_id, aggregate_type, event_type, event_version, payload, occurred_at)
		 VALUES ($1, $2, 'Order', $3, 1, $4::jsonb, $5)`, id, uuid.New(), eventType, payload, now)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testSettings(baseURL string) catalog.Settings {
	settings := bootstrap.CatalogSettings(baseURL)
	settings.BreakerMinRequests = 100
	return settings
}

func clearTables(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE outbox, idempotency_keys, order_items, orders"); err != nil {
		t.Fatal(err)
	}
}

type fakeCatalog struct {
	URL    string
	mu     sync.Mutex
	hits   int
	script func(hit int, w http.ResponseWriter, r *http.Request)
}

func startCatalog(t *testing.T, script func(hit int, w http.ResponseWriter, r *http.Request)) *fakeCatalog {
	t.Helper()
	fake := &fakeCatalog{script: script}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		fake.hits++
		hit := fake.hits
		fake.mu.Unlock()
		fake.script(hit, w, r)
	}))
	t.Cleanup(server.Close)
	fake.URL = server.URL
	return fake
}

func (f *fakeCatalog) Hits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits
}

func answerPrice(w http.ResponseWriter, r *http.Request, price string) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/products/")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":"%s","title":"Кофемолка","price":%s,"currency":"RUB","status":"PUBLISHED"}`, id, price)
}

func answerNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `{"code":"PRODUCT_NOT_FOUND","status":404}`)
}

func holdFor(r *http.Request, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-r.Context().Done():
		return false
	}
}

func dropConnection(w http.ResponseWriter) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	_ = conn.Close()
}

func customerToken(id uuid.UUID) string { return "customer." + id.String() }
func adminToken(id uuid.UUID) string    { return "admin." + id.String() }

func orderBody(productID, sellerID uuid.UUID, quantity int) string {
	return fmt.Sprintf(`{"items":[{"productId":"%s","sellerId":"%s","quantity":%d}],
		"shippingAddress":{"country":"RU","city":"Москва","street":"Тверская, 1","postalCode":"125009"}}`,
		productID, sellerID, quantity)
}

func postOrder(t *testing.T, router http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	return postOrderWithKey(t, router, token, uuid.NewString(), body)
}

func postOrderWithKey(t *testing.T, router http.Handler, token, idempotencyKey, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func call(t *testing.T, router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("ответ не JSON-объект: %s", rec.Body.String())
	}
	return out
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("ожидали %d, получили %d: %s", want, rec.Code, rec.Body.String())
	}
}

func expectCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if got := decode(t, rec)["code"]; got != want {
		t.Fatalf("ожидали код %s, получили %v: %s", want, got, rec.Body.String())
	}
}

func ordersInDB(t *testing.T) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM orders").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func expectOrders(t *testing.T, want int) {
	t.Helper()
	if got := ordersInDB(t); got != want {
		t.Fatalf("в базе %d заказов, ожидали %d", got, want)
	}
}

func expectHits(t *testing.T, fake *fakeCatalog, want int) {
	t.Helper()
	if got := fake.Hits(); got != want {
		t.Fatalf("к каталогу ушло %d запросов, ожидали %d", got, want)
	}
}
