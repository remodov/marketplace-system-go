package payment_test

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
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/remodov/marketplace-system-go/services/payment/internal/payment"
)

var (
	db     *sql.DB
	router http.Handler
	now    = time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return now }

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://catalog:catalog@localhost:5440/payments_test?sslmode=disable"
	}
	var err error
	db, err = sql.Open("pgx", url)
	if err != nil {
		log.Fatal(err)
	}
	if err := payment.EnsureSchema(context.Background(), db); err != nil {
		log.Fatalf("тестовая база недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", err)
	}
	router = payment.NewRouter(payment.NewService(db, fixedClock{}), db)
	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

func clean(t *testing.T) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM payments"); err != nil {
		t.Fatal(err)
	}
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

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("ответ не JSON: %s", rec.Body.String())
	}
	return out
}

func expect(t *testing.T, rec *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("ожидали %d, получили %d: %s", status, rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

func authorize(t *testing.T, orderID uuid.UUID) string {
	t.Helper()
	body := fmt.Sprintf(`{"orderId":"%s","amount":1990.00,"currency":"RUB"}`, orderID)
	return expect(t, call(t, http.MethodPost, "/api/v1/payments", body), http.StatusCreated)["id"].(string)
}

func countPayments(t *testing.T) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM payments").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestAuthorize_createsPaymentInAuthorized(t *testing.T) {
	clean(t)
	orderID := uuid.New()
	id := authorize(t, orderID)

	body := expect(t, call(t, http.MethodGet, "/api/v1/payments/"+id, ""), http.StatusOK)
	if body["status"] != "AUTHORIZED" || body["orderId"] != orderID.String() || body["amount"] != 1990.0 {
		t.Fatalf("неожиданный платёж: %v", body)
	}
}

func TestAuthorize_isIdempotentPerOrder(t *testing.T) {
	clean(t)
	orderID := uuid.New()

	first := authorize(t, orderID)
	second := authorize(t, orderID)

	if first != second || countPayments(t) != 1 {
		t.Fatalf("повторная авторизация обязана вернуть прежний платёж: %s и %s, в базе %d", first, second, countPayments(t))
	}
}

func TestCapture_movesToCaptured(t *testing.T) {
	clean(t)
	id := authorize(t, uuid.New())

	body := expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/capture", ""), http.StatusOK)
	if body["status"] != "CAPTURED" {
		t.Fatalf("ожидали CAPTURED: %v", body)
	}
}

func TestRefund_afterCaptureIsAllowed(t *testing.T) {
	clean(t)
	id := authorize(t, uuid.New())
	expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/capture", ""), http.StatusOK)

	body := expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/refund", ""), http.StatusOK)
	if body["status"] != "REFUNDED" {
		t.Fatalf("ожидали REFUNDED: %v", body)
	}
}

func TestRefund_repeatIsSafe(t *testing.T) {
	clean(t)
	id := authorize(t, uuid.New())
	first := expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/refund", ""), http.StatusOK)

	second := expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/refund", ""), http.StatusOK)

	if second["status"] != "REFUNDED" || second["updatedAt"] != first["updatedAt"] {
		t.Fatalf("повторный возврат это тот же ответ, а не второй возврат: %v и %v", first, second)
	}
}

func TestCapture_afterRefundIsRejected(t *testing.T) {
	clean(t)
	id := authorize(t, uuid.New())
	expect(t, call(t, http.MethodPost, "/api/v1/payments/"+id+"/refund", ""), http.StatusOK)

	rec := call(t, http.MethodPost, "/api/v1/payments/"+id+"/capture", "")

	if body := expect(t, rec, http.StatusConflict); body["code"] != "INVALID_PAYMENT_TRANSITION" {
		t.Fatalf("ожидали INVALID_PAYMENT_TRANSITION: %v", body)
	}
	if body := expect(t, call(t, http.MethodGet, "/api/v1/payments/"+id, ""), http.StatusOK); body["status"] != "REFUNDED" {
		t.Fatalf("запрещённый переход не должен портить данные: %v", body)
	}
}

func TestPayment_unknownIsNotFound(t *testing.T) {
	expect(t, call(t, http.MethodGet, "/api/v1/payments/"+uuid.NewString(), ""), http.StatusNotFound)
}

func TestAuthorize_zeroAmountIsRejected(t *testing.T) {
	body := fmt.Sprintf(`{"orderId":"%s","amount":0,"currency":"RUB"}`, uuid.New())
	expect(t, call(t, http.MethodPost, "/api/v1/payments", body), http.StatusBadRequest)
}
