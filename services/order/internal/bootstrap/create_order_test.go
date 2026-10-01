package bootstrap_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
)

func TestCreateOrder_whenCatalogAnswers_storesOrderWithCatalogPrices(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "2490.50") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, seller, product := uuid.New(), uuid.New(), uuid.New()

	rec := postOrder(t, router, customerToken(customer), orderBody(product, seller, 2))

	expectStatus(t, rec, http.StatusCreated)
	body := decode(t, rec)
	if body["status"] != "DRAFT" || body["total"] != 4981.0 {
		t.Fatalf("ожидали черновик на 4981.00, получили %s", rec.Body.String())
	}
	var status, total, unitPrice string
	err := pool.QueryRow(context.Background(),
		`SELECT o.status::text, o.total_amount::text, i.unit_price::text FROM orders o JOIN order_items i ON i.order_id = o.id WHERE o.id = $1`,
		body["id"]).Scan(&status, &total, &unitPrice)
	if err != nil {
		t.Fatal(err)
	}
	if status != "DRAFT" || total != "4981.00" || unitPrice != "2490.50" {
		t.Fatalf("в базе %s %s %s", status, total, unitPrice)
	}
	expectHits(t, fake, 1)
}

func TestCreateOrder_whenProductUnknown_returns404WithoutRetry(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, _ *http.Request) { answerNotFound(w) })
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusNotFound)
	expectCode(t, rec, "PRODUCT_NOT_FOUND")
	expectOrders(t, 0)
	expectHits(t, fake, 1)
}

func TestCreateOrder_whenTwoSellers_isRejectedBeforeCatalog(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	body := `{"items":[{"productId":"` + uuid.NewString() + `","sellerId":"` + uuid.NewString() + `","quantity":1},
		{"productId":"` + uuid.NewString() + `","sellerId":"` + uuid.NewString() + `","quantity":1}],
		"shippingAddress":{"country":"RU","city":"Москва","street":"Тверская, 1","postalCode":"125009"}}`

	rec := postOrder(t, router, customerToken(uuid.New()), body)

	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "MULTI_SELLER_NOT_SUPPORTED")
	expectOrders(t, 0)
	expectHits(t, fake, 0)
}

func TestCreateOrder_whenAnonymous_isRejected(t *testing.T) {
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := postOrder(t, router, "", orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusUnauthorized)
	expectCode(t, rec, "TOKEN_MISSING")
}

func TestGetOrder_whenForeignCustomer_returns404ButAdminSees(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "100.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	owner := uuid.New()
	created := decode(t, postOrder(t, router, customerToken(owner), orderBody(uuid.New(), uuid.New(), 1)))
	path := "/api/v1/orders/" + created["id"].(string)

	expectStatus(t, call(t, router, http.MethodGet, path, customerToken(owner), ""), http.StatusOK)
	foreign := call(t, router, http.MethodGet, path, customerToken(uuid.New()), "")
	expectStatus(t, foreign, http.StatusNotFound)
	expectCode(t, foreign, "ORDER_NOT_FOUND")
	expectStatus(t, call(t, router, http.MethodGet, path, adminToken(uuid.New()), ""), http.StatusOK)
}
