package product_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/migrations"
	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/product"
)

var (
	pool    *pgxpool.Pool
	service *product.Service
	router  http.Handler
)

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://catalog:catalog@localhost:5440/catalog_starter_test?sslmode=disable"
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", url)
	if err == nil {
		err = migrations.Up(ctx, db)
	}
	if err != nil {
		log.Fatalf("тестовая база недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", err)
	}
	_ = db.Close()
	pool, err = pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	service = product.NewService(product.NewRepository(pool))
	r := chi.NewRouter()
	product.Routes(r, service)
	router = r
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func call(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("ответ не JSON-объект: %s", rec.Body.String())
	}
	return out
}

func list(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("ответ не JSON-список: %s", rec.Body.String())
	}
	return out
}

func mustCreate(t *testing.T, title string, price string, stock int) *product.Product {
	t.Helper()
	p, err := service.Create(context.Background(), title, decimal.RequireFromString(price), stock)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func unique(title string) string {
	return fmt.Sprintf("%s %s", title, uuid.NewString()[:8])
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("ожидали %d, получили %d: %s", want, rec.Code, rec.Body.String())
	}
}

func TestCreatedProductIsReturnedByID(t *testing.T) {
	rec := call(t, http.MethodPost, "/products", `{"title":"Беспроводная мышь","price":1990.00,"stock":7}`)
	expectStatus(t, rec, http.StatusCreated)
	created := body(t, rec)
	if created["title"] != "Беспроводная мышь" {
		t.Fatalf("название в ответе: %v", created["title"])
	}
	rec = call(t, http.MethodGet, "/products/"+created["id"].(string), "")
	expectStatus(t, rec, http.StatusOK)
	if got := body(t, rec)["stock"]; got != float64(7) {
		t.Fatalf("остаток: %v", got)
	}
}

func TestSearchFindsByPartOfTitle(t *testing.T) {
	title := unique("Механическая клавиатура")
	mustCreate(t, title, "5400.00", 3)
	rec := call(t, http.MethodGet, "/products?query="+strings.ReplaceAll(title[len("Механическая "):], " ", "%20"), "")
	expectStatus(t, rec, http.StatusOK)
	found := list(t, rec)
	if len(found) != 1 || found[0]["title"] != title {
		t.Fatalf("поиск по части названия: %v", found)
	}
}

func TestReserveHoldsStockInsteadOfWritingItOff(t *testing.T) {
	p := mustCreate(t, unique("USB-хаб"), "890.00", 5)
	rec := call(t, http.MethodPost, "/products/"+p.ID().String()+"/reserve", `{"quantity":2}`)
	expectStatus(t, rec, http.StatusOK)
	card := body(t, rec)
	if card["stock"] != float64(5) || card["reserved"] != float64(2) || card["available"] != float64(3) {
		t.Fatalf("резерв удерживает, а не списывает: stock=%v reserved=%v available=%v",
			card["stock"], card["reserved"], card["available"])
	}
}

func TestReserveMoreThanStockIsRejected(t *testing.T) {
	p := mustCreate(t, unique("Коврик"), "450.00", 1)
	rec := call(t, http.MethodPost, "/products/"+p.ID().String()+"/reserve", `{"quantity":4}`)
	expectStatus(t, rec, http.StatusConflict)
}

func TestUnknownProductGivesNotFound(t *testing.T) {
	rec := call(t, http.MethodGet, "/products/"+uuid.NewString(), "")
	expectStatus(t, rec, http.StatusNotFound)
	if detail, _ := body(t, rec)["detail"].(string); !strings.Contains(detail, "не найден") {
		t.Fatalf("тело 404: %s", rec.Body.String())
	}
}
