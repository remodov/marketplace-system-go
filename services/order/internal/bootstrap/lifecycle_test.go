package bootstrap_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	paymentsv1 "github.com/remodov/marketplace-system-go/contracts/payments/v1"
	kafkain "github.com/remodov/marketplace-system-go/services/order/internal/adapter/in/kafka"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/payment"
	"github.com/remodov/marketplace-system-go/services/order/internal/bootstrap"
)

type scenario struct {
	app      *bootstrap.App
	router   http.Handler
	payment  *fakePayment
	clock    *testClock
	customer uuid.UUID
	seller   uuid.UUID
	orderID  string
}

func givenOrder(t *testing.T, price string) *scenario {
	t.Helper()
	clearTables(t)
	fake := startCatalog(t, func(_ int, w http.ResponseWriter, r *http.Request) { answerPrice(w, r, price) })
	pay := startPayment(t)
	clock := &testClock{now: now}
	app := newAppFull(t, bootstrap.Deps{Clock: clock, Catalog: catalog.New(testSettings(fake.URL)), Payment: payment.New(bootstrap.PaymentSettings(pay.URL))})
	s := &scenario{app: app, router: app.Handler, payment: pay, clock: clock, customer: uuid.New(), seller: uuid.New()}
	created := postOrder(t, s.router, customerToken(s.customer), orderBody(uuid.New(), s.seller, 1))
	expectStatus(t, created, http.StatusCreated)
	s.orderID = decode(t, created)["id"].(string)
	return s
}

func (s *scenario) path(action string) string { return "/api/v1/orders/" + s.orderID + action }

func (s *scenario) confirm(t *testing.T) *scenario {
	t.Helper()
	expectStatus(t, postJSON(t, s.router, s.path("/confirm"), customerToken(s.customer), ""), http.StatusOK)
	return s
}

func (s *scenario) pay(t *testing.T, paymentID uuid.UUID) *scenario {
	t.Helper()
	expectStatus(t, postJSON(t, s.router, s.path("/pay"), adminToken(uuid.New()), `{"paymentId":"`+paymentID.String()+`"}`), http.StatusOK)
	return s
}

func TestLifecycle_fullPathFromDraftToDelivered(t *testing.T) {
	s := givenOrder(t, "200.00")
	paymentID := uuid.New()

	confirmed := decode(t, postJSON(t, s.router, s.path("/confirm"), customerToken(s.customer), ""))
	paid := decode(t, postJSON(t, s.router, s.path("/pay"), adminToken(uuid.New()), `{"paymentId":"`+paymentID.String()+`"}`))
	shipped := decode(t, postJSON(t, s.router, s.path("/ship"), sellerToken(s.seller), `{"trackingNumber":"TRACK-12345"}`))
	delivered := decode(t, postJSON(t, s.router, s.path("/deliver"), customerToken(s.customer), ""))

	if confirmed["status"] != "PENDING_PAYMENT" || paid["status"] != "PAID" || shipped["status"] != "SHIPPED" || delivered["status"] != "DELIVERED" {
		t.Fatalf("статусы по пути: %v %v %v %v", confirmed["status"], paid["status"], shipped["status"], delivered["status"])
	}
	if paid["paymentId"] != paymentID.String() || paid["paidAt"] == nil || shipped["shippedAt"] == nil || delivered["deliveredAt"] == nil {
		t.Fatalf("отметки времени и платёж должны доезжать до ответа: %v", delivered)
	}
	types := eventTypes(t)
	for _, want := range []string{"OrderCreated", "OrderConfirmed", "OrderPaid", "OrderShipped", "OrderDelivered"} {
		if !slices.Contains(types, want) {
			t.Fatalf("в outbox нет %s: %v", want, types)
		}
	}
}

func TestLifecycle_payIsIdempotentForTheSamePayment(t *testing.T) {
	s := givenOrder(t, "200.00").confirm(t)
	paymentID := uuid.New()

	s.pay(t, paymentID).pay(t, paymentID)

	if countEvents(t, "OrderPaid") != 1 {
		t.Fatalf("повторный вебхук с тем же платежом не должен рождать второе событие: %v", eventTypes(t))
	}
}

func TestLifecycle_payFromDraftIsInvalidState(t *testing.T) {
	s := givenOrder(t, "200.00")

	rec := postJSON(t, s.router, s.path("/pay"), adminToken(uuid.New()), `{"paymentId":"`+uuid.NewString()+`"}`)

	expectStatus(t, rec, http.StatusConflict)
	expectCode(t, rec, "ORDER_INVALID_STATE")
}

func TestLifecycle_confirmBelowMinimumIsRejected(t *testing.T) {
	s := givenOrder(t, "50.00")

	rec := postJSON(t, s.router, s.path("/confirm"), customerToken(s.customer), "")

	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "ORDER_BELOW_MINIMUM")
	if countEvents(t, "OrderConfirmed") != 0 {
		t.Fatal("отклонённое подтверждение не должно рождать событие")
	}
}

func TestLifecycle_foreignSellerCannotShipAndDeliverNeedsShipped(t *testing.T) {
	s := givenOrder(t, "200.00").confirm(t).pay(t, uuid.New())

	foreign := postJSON(t, s.router, s.path("/ship"), sellerToken(uuid.New()), `{"trackingNumber":"TRACK-99"}`)
	expectStatus(t, foreign, http.StatusNotFound)
	expectCode(t, foreign, "ORDER_NOT_FOUND")

	early := postJSON(t, s.router, s.path("/deliver"), customerToken(s.customer), "")
	expectStatus(t, early, http.StatusConflict)
	expectCode(t, early, "ORDER_INVALID_STATE")
}

func TestLifecycle_cancelPaidOrderRefundsThroughPayment(t *testing.T) {
	paymentID := uuid.New()
	s := givenOrder(t, "200.00").confirm(t).pay(t, paymentID)

	rec := postJSON(t, s.router, s.path("/cancel"), customerToken(s.customer), `{"reasonCode":"changed_mind","comment":"передумал"}`)

	expectStatus(t, rec, http.StatusOK)
	if decode(t, rec)["status"] != "CANCELLED" {
		t.Fatalf("ожидали CANCELLED: %s", rec.Body.String())
	}
	requests := s.payment.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/api/v1/payments/"+paymentID.String()+"/refund" || requests[0].Header.Get("Idempotency-Key") != "refund-"+s.orderID {
		t.Fatalf("в платежи должен уйти один возврат с ключом идемпотентности: %+v", requests)
	}
	var cancelled ordersv1.OrderCancelledPayload
	for _, row := range outboxRows(t) {
		if row.EventType == "OrderCancelled" {
			_ = json.Unmarshal([]byte(row.Payload), &cancelled)
		}
	}
	if cancelled.PreviousStatus != "PAID" || cancelled.Reason != "CHANGED_MIND" || cancelled.RefundID == nil || *cancelled.RefundID != paymentID {
		t.Fatalf("OrderCancelled должен нести прежний статус, причину и возврат: %+v", cancelled)
	}
}

func TestLifecycle_cancelPaidOrderWhenPaymentIsDownKeepsItPaid(t *testing.T) {
	s := givenOrder(t, "200.00").confirm(t).pay(t, uuid.New())
	s.payment.GoDown()

	rec := postJSON(t, s.router, s.path("/cancel"), customerToken(s.customer), `{"reasonCode":"changed_mind"}`)

	expectStatus(t, rec, http.StatusServiceUnavailable)
	expectCode(t, rec, "SERVICE_DEGRADED")
	current := decode(t, call(t, s.router, http.MethodGet, s.path(""), customerToken(s.customer), ""))
	if current["status"] != "PAID" || countEvents(t, "OrderCancelled") != 0 {
		t.Fatalf("без возврата отмены быть не должно: %v, события %v", current["status"], eventTypes(t))
	}
}

func TestLifecycle_cancelDraftNeedsNoRefund(t *testing.T) {
	s := givenOrder(t, "200.00")

	rec := postJSON(t, s.router, s.path("/cancel"), customerToken(s.customer), `{"reasonCode":"mistake"}`)

	expectStatus(t, rec, http.StatusOK)
	if len(s.payment.Requests()) != 0 {
		t.Fatal("черновик отменяется без похода в платежи")
	}
}

func TestLifecycle_unpaidOrderExpiresAfterTimeout(t *testing.T) {
	s := givenOrder(t, "200.00").confirm(t)

	s.clock.Advance(14 * time.Minute)
	if expired, _ := s.app.Expirer.Once(context.Background()); expired != 0 {
		t.Fatalf("до таймаута ничего не истекает, закрыто %d", expired)
	}
	s.clock.Advance(2 * time.Minute)
	expired, err := s.app.Expirer.Once(context.Background())
	if err != nil || expired != 1 {
		t.Fatalf("ожидали один просроченный заказ: %d, %v", expired, err)
	}

	current := decode(t, call(t, s.router, http.MethodGet, s.path(""), customerToken(s.customer), ""))
	if current["status"] != "EXPIRED" || countEvents(t, "OrderExpired") != 1 {
		t.Fatalf("заказ должен закрыться по таймауту: %v, события %v", current["status"], eventTypes(t))
	}
}

func TestPaymentConsumer_paymentCompletedMarksPaidOnce(t *testing.T) {
	s := givenOrder(t, "200.00").confirm(t)
	paymentID, eventID := uuid.New(), uuid.New()
	payload, _ := json.Marshal(paymentsv1.PaymentCompletedPayload{PaymentID: paymentID, OrderID: uuid.MustParse(s.orderID), Amount: "200.00", Currency: "RUB", OccurredAt: now})
	message := kafkago.Message{Value: payload, Headers: []kafkago.Header{
		{Key: ordersv1.HeaderEventID, Value: []byte(eventID.String())},
		{Key: ordersv1.HeaderEventType, Value: []byte(paymentsv1.EventPaymentCompleted)},
	}}
	handler := kafkain.NewPaymentHandler(s.app.Lifecycle)

	if err := handler.Handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}

	current := decode(t, call(t, s.router, http.MethodGet, s.path(""), customerToken(s.customer), ""))
	if current["status"] != "PAID" || current["paymentId"] != paymentID.String() || countEvents(t, "OrderPaid") != 1 {
		t.Fatalf("событие платежа переводит заказ в PAID ровно один раз: %v, события %v", current["status"], eventTypes(t))
	}
}
