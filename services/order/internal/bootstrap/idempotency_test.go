package bootstrap_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
)

func TestIdempotency_sameKeySameBody_returnsSameOrder(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "200.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, key, body := uuid.New(), uuid.NewString(), orderBody(uuid.New(), uuid.New(), 1)

	first := postOrderWithKey(t, router, customerToken(customer), key, body)
	second := postOrderWithKey(t, router, customerToken(customer), key, body)

	expectStatus(t, first, http.StatusCreated)
	expectStatus(t, second, http.StatusOK)
	if decode(t, first)["id"] != decode(t, second)["id"] {
		t.Fatalf("повтор должен вернуть тот же заказ: %s и %s", first.Body.String(), second.Body.String())
	}
	expectOrders(t, 1)
	expectHits(t, fake, 1)
}

func TestIdempotency_sameKeyDifferentBody_isConflict(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "200.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, key, product, seller := uuid.New(), uuid.NewString(), uuid.New(), uuid.New()

	expectStatus(t, postOrderWithKey(t, router, customerToken(customer), key, orderBody(product, seller, 1)), http.StatusCreated)
	rec := postOrderWithKey(t, router, customerToken(customer), key, orderBody(product, seller, 5))

	expectStatus(t, rec, http.StatusConflict)
	expectCode(t, rec, "IDEMPOTENCY_KEY_CONFLICT")
	expectOrders(t, 1)
}

func TestIdempotency_differentKeys_createDifferentOrders(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "200.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, body := uuid.New(), orderBody(uuid.New(), uuid.New(), 1)

	expectStatus(t, postOrder(t, router, customerToken(customer), body), http.StatusCreated)
	expectStatus(t, postOrder(t, router, customerToken(customer), body), http.StatusCreated)

	expectOrders(t, 2)
}

func TestIdempotency_sameKeyAtOnce_createsOneOrder(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "200.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))
	customer, key, body := uuid.New(), uuid.NewString(), orderBody(uuid.New(), uuid.New(), 1)

	const clients = 8
	ids := make([]any, clients)
	var wg sync.WaitGroup
	for i := range clients {
		wg.Go(func() {
			rec := postOrderWithKey(t, router, customerToken(customer), key, body)
			if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
				t.Errorf("клиент %d получил %d: %s", i, rec.Code, rec.Body.String())
				return
			}
			ids[i] = decode(t, rec)["id"]
		})
	}
	wg.Wait()

	for i := 1; i < clients; i++ {
		if ids[i] != ids[0] {
			t.Fatalf("клиенты получили разные заказы: %v", ids)
		}
	}
	expectOrders(t, 1)
}

func TestCreateOrder_withoutIdempotencyKey_isRejected(t *testing.T) {
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, "200.00") })
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := call(t, router, http.MethodPost, "/api/v1/orders", customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "VALIDATION_ERROR")
}
