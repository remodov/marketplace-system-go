package bootstrap_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
)

func TestCatalog_whenFirstAnswerHangs_retrySavesTheOrder(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(hit int, w http.ResponseWriter, r *http.Request) {
		if hit == 1 && !holdFor(r, 2500*time.Millisecond) {
			return
		}
		answerPrice(w, r, "100.00")
	})
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusCreated)
	expectOrders(t, 1)
	expectHits(t, fake, 2)
}

func TestCatalog_whenDown_orderIsNotCreatedAndErrorIsDomain(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, _ *http.Request) { dropConnection(w) })
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusServiceUnavailable)
	expectCode(t, rec, "SERVICE_DEGRADED")
	expectOrders(t, 0)
	expectHits(t, fake, 2)
}

func TestCatalog_whenSlow_isCutOffByTimeout(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		if holdFor(r, 4*time.Second) {
			answerPrice(w, r, "100.00")
		}
	})
	router := newApp(t, catalog.New(testSettings(fake.URL)))

	started := time.Now()
	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))
	spent := time.Since(started)

	expectStatus(t, rec, http.StatusServiceUnavailable)
	expectCode(t, rec, "SERVICE_DEGRADED")
	if spent >= 4*time.Second {
		t.Fatalf("ждали ответа %s: обе попытки должны упереться в таймаут раньше", spent)
	}
	expectOrders(t, 0)
}

func TestCatalog_whenDownRepeatedly_breakerStopsCallingIt(t *testing.T) {
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, _ *http.Request) { dropConnection(w) })
	settings := testSettings(fake.URL)
	settings.BreakerMinRequests = 3
	router := newApp(t, catalog.New(settings))

	for range 3 {
		rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))
		expectStatus(t, rec, http.StatusServiceUnavailable)
	}
	expectHits(t, fake, 6)

	rec := postOrder(t, router, customerToken(uuid.New()), orderBody(uuid.New(), uuid.New(), 1))

	expectStatus(t, rec, http.StatusServiceUnavailable)
	expectCode(t, rec, "SERVICE_DEGRADED")
	expectHits(t, fake, 6)
	expectOrders(t, 0)
}
