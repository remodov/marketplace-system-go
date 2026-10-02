package bootstrap_test

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/bootstrap"
)

var (
	pool   *pgxpool.Pool
	router http.Handler
	now    = time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://catalog:catalog@localhost:5440/catalog_test?sslmode=disable"
	}
	ctx := context.Background()
	if err := bootstrap.Migrate(ctx, url); err != nil {
		log.Fatalf("тестовая база недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", err)
	}
	cfg := bootstrap.FromEnv()
	cfg.DatabaseURL = url
	cfg.AuthMode = "local"
	app, err := bootstrap.Build(ctx, cfg, bootstrap.Deps{Clock: fixedClock{}})
	if err != nil {
		log.Fatal(err)
	}
	pool = app.Pool
	router = app.Handler
	code := m.Run()
	app.Close()
	os.Exit(code)
}

func clearTables(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE catalog_audit_log, products"); err != nil {
		t.Fatal(err)
	}
}

func givenProduct(t *testing.T, seller uuid.UUID, status, price string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO products (id, title, description, price, currency, seller_id, status, created_at, updated_at)
		 VALUES ($1, 'Ноутбук', 'Тестовый товар', $2::numeric, 'RUB', $3, $4::product_status, $5, $5)`,
		id, price, seller, status, now)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func priceInDB(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var price string
	if err := pool.QueryRow(context.Background(), "SELECT price::text FROM products WHERE id = $1", id).Scan(&price); err != nil {
		t.Fatal(err)
	}
	return price
}

func auditActions(t *testing.T) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT action FROM catalog_audit_log ORDER BY occurred_at")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a)
	}
	return actions
}

func sellerToken(id uuid.UUID) string { return "seller." + id.String() }
func adminToken(id uuid.UUID) string  { return "admin." + id.String() }

func call(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
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
