package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/remodov/marketplace-system-go/services/bff/internal/httpapi"
	"github.com/remodov/marketplace-system-go/services/bff/internal/ratelimit"
	"github.com/remodov/marketplace-system-go/services/bff/internal/screen"
)

type stubs struct {
	order, catalog, payment                *httptest.Server
	orderCalls, catalogCalls, paymentCalls atomic.Int32
	orderID, productID, paymentID          uuid.UUID
	paymentStatus                          int
}

func newStubs(t *testing.T) *stubs {
	t.Helper()
	s := &stubs{orderID: uuid.New(), productID: uuid.New(), paymentID: uuid.New(), paymentStatus: http.StatusOK}
	s.order = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.orderCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + s.orderID.String() + `","status":"PAID","total":"3980.00","paymentId":"` + s.paymentID.String() + `","items":[{"productId":"` + s.productID.String() + `","quantity":2}]}`))
	}))
	s.catalog = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.catalogCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + s.productID.String() + `","title":"Беспроводная мышь","price":1990.00,"currency":"RUB"}`))
	}))
	s.payment = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.paymentCalls.Add(1)
		if s.paymentStatus != http.StatusOK {
			w.WriteHeader(s.paymentStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + s.paymentID.String() + `","status":"CAPTURED"}`))
	}))
	t.Cleanup(func() { s.order.Close(); s.catalog.Close(); s.payment.Close() })
	return s
}

func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6381"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("Redis недоступен (%v): подними стенд командой docker compose -f infra/compose.yaml up -d redis", err)
	}
	return client
}

func newRouter(t *testing.T, s *stubs, perMinute int) http.Handler {
	t.Helper()
	return httpapi.NewRouter(ratelimit.NewLimiter(redisClient(t), perMinute), screen.NewAssembler(s.order.URL, s.catalog.URL, s.payment.URL))
}

func get(router http.Handler, path, client string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Client-Id", client)
	req.Header.Set("Authorization", "Bearer customer."+uuid.New().String())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestScreen_assembledFromThreeServices(t *testing.T) {
	s := newStubs(t)
	router := newRouter(t, s, 60)

	rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), uuid.New().String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var body screen.OrderScreen
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "PAID" || body.PaymentStatus != "CAPTURED" {
		t.Fatalf("статусы не те: %+v", body)
	}
	if len(body.Items) != 1 || body.Items[0].Title != "Беспроводная мышь" || body.Items[0].Quantity != 2 || !body.Items[0].Price.Equal(decimalOf("1990.00")) {
		t.Fatalf("строка экрана не та: %+v", body.Items)
	}
	if s.orderCalls.Load() != 1 || s.catalogCalls.Load() != 1 || s.paymentCalls.Load() != 1 {
		t.Fatalf("ожидали по одному походу к каждому соседу, получили order=%d catalog=%d payment=%d", s.orderCalls.Load(), s.catalogCalls.Load(), s.paymentCalls.Load())
	}
}

func TestScreen_survivesMissingPayment(t *testing.T) {
	s := newStubs(t)
	s.paymentStatus = http.StatusNotFound
	router := newRouter(t, s, 60)

	rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), uuid.New().String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var body screen.OrderScreen
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.PaymentStatus != screen.PaymentNone || len(body.Items) != 1 || body.Items[0].Title != "Беспроводная мышь" {
		t.Fatalf("экран без платежа собран неверно: %+v", body)
	}
}

func TestScreen_downstreamDownIsBadGateway(t *testing.T) {
	s := newStubs(t)
	router := newRouter(t, s, 60)
	s.catalog.Close()

	rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), uuid.New().String())

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("ожидали 502, получили %d: %s", rec.Code, rec.Body.String())
	}
	var problem map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &problem)
	if problem["code"] != "DOWNSTREAM_UNAVAILABLE" {
		t.Fatalf("ожидали код DOWNSTREAM_UNAVAILABLE: %s", rec.Body.String())
	}
}

func TestRateLimit_fourthRequestInAMinuteIsRejected(t *testing.T) {
	s := newStubs(t)
	router := newRouter(t, s, 3)
	client := uuid.New().String()

	for i := 0; i < 3; i++ {
		if rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), client); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("запрос %d не должен упираться в лимит", i+1)
		}
	}
	rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), client)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("четвёртый запрос должен получить 429, получили %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("при 429 нужен Retry-After")
	}
}

func TestRateLimit_countsPerClient(t *testing.T) {
	s := newStubs(t)
	router := newRouter(t, s, 3)
	noisy, quiet := uuid.New().String(), uuid.New().String()

	for i := 0; i < 4; i++ {
		get(router, "/api/v1/screens/order/"+s.orderID.String(), noisy)
	}
	rec := get(router, "/api/v1/screens/order/"+s.orderID.String(), quiet)

	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("сосед не должен расходовать чужую квоту")
	}
}
