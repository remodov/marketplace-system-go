package product_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/cache"
	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/product"
)

type countingStore struct {
	product.Store
	byID atomic.Int32
}

func (c *countingStore) ByID(ctx context.Context, id uuid.UUID) (*product.Product, error) {
	c.byID.Add(1)
	return c.Store.ByID(ctx, id)
}

func cachedStand(t *testing.T) (*countingStore, *product.Service, http.Handler) {
	t.Helper()
	counting := &countingStore{Store: product.NewRepository(pool)}
	svc := product.NewService(counting, cache.NewMemory(10*time.Minute))
	r := chi.NewRouter()
	product.Routes(r, svc)
	return counting, svc, r
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func patch(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRepeatedRequestIsServedFromCache(t *testing.T) {
	counting, svc, h := cachedStand(t)
	p, err := svc.Create(context.Background(), unique("Мышь для кэша"), decimal.RequireFromString("1990.00"), 5)
	if err != nil {
		t.Fatal(err)
	}
	counting.byID.Store(0)
	for i := 0; i < 3; i++ {
		expectStatus(t, get(t, h, "/products/"+p.ID().String()), http.StatusOK)
	}
	if got := counting.byID.Load(); got != 1 {
		t.Fatalf("три запроса карточки должны дать одно обращение к базе, а дали %d", got)
	}
}

func TestPriceChangeDropsTheCache(t *testing.T) {
	_, svc, h := cachedStand(t)
	p, err := svc.Create(context.Background(), unique("Мышь для кэша"), decimal.RequireFromString("1990.00"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := body(t, get(t, h, "/products/"+p.ID().String()))["price"]; got != float64(1990) {
		t.Fatalf("цена до правки: %v", got)
	}
	expectStatus(t, patch(t, h, "/products/"+p.ID().String()+"/price", `{"price":1490.00}`), http.StatusOK)
	if got := body(t, get(t, h, "/products/"+p.ID().String()))["price"]; got != float64(1490) {
		t.Fatalf("после смены цены карточка должна обновиться, а в ней %v", got)
	}
}

func TestReserveDropsTheCache(t *testing.T) {
	_, svc, h := cachedStand(t)
	p, err := svc.Create(context.Background(), unique("Мышь для кэша"), decimal.RequireFromString("1990.00"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := body(t, get(t, h, "/products/"+p.ID().String()))["available"]; got != float64(5) {
		t.Fatalf("доступно до резерва: %v", got)
	}
	if _, err := svc.Reserve(context.Background(), p.ID(), 2); err != nil {
		t.Fatal(err)
	}
	card := body(t, get(t, h, "/products/"+p.ID().String()))
	if card["available"] != float64(3) || card["reserved"] != float64(2) {
		t.Fatalf("после резерва карточка должна обновиться: %v", card)
	}
}
