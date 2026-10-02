package screen

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const PaymentNone = "NONE"

type Item struct {
	ProductID uuid.UUID       `json:"productId"`
	Title     string          `json:"title"`
	Quantity  int             `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
}

type OrderScreen struct {
	OrderID       uuid.UUID       `json:"orderId"`
	Status        string          `json:"status"`
	Total         decimal.Decimal `json:"total"`
	PaymentStatus string          `json:"paymentStatus"`
	Items         []Item          `json:"items"`
}

type orderLine struct {
	ProductID uuid.UUID `json:"productId"`
	Quantity  int       `json:"quantity"`
}

type orderResponse struct {
	ID        uuid.UUID       `json:"id"`
	Status    string          `json:"status"`
	Total     decimal.Decimal `json:"total"`
	PaymentID *uuid.UUID      `json:"paymentId"`
	Items     []orderLine     `json:"items"`
}

type productCard struct {
	Title string          `json:"title"`
	Price decimal.Decimal `json:"price"`
}

type paymentResponse struct {
	Status string `json:"status"`
}

type Assembler struct {
	order   *client
	catalog *client
	payment *client
}

func NewAssembler(orderURL, catalogURL, paymentURL string) *Assembler {
	return &Assembler{
		order:   newClient("order", orderURL),
		catalog: newClient("catalog", catalogURL),
		payment: newClient("payment", paymentURL),
	}
}

func (a *Assembler) Assemble(ctx context.Context, orderID uuid.UUID, authorization string) (OrderScreen, error) {
	var order orderResponse
	if _, err := a.order.getJSON(ctx, "/api/v1/orders/"+orderID.String(), authorization, &order); err != nil {
		return OrderScreen{}, err
	}

	// TODO шаг 13: собрать экран.
	// Заказ уже прочитан - из него известны товары и идентификатор платежа. Осталось
	// добрать карточки товаров (a.catalog) и статус платежа (a.paymentStatus).
	// Эти походы независимы, и экран не обязан ждать их по очереди.
	_ = order
	return OrderScreen{}, errors.New("шаг 13: экран ещё не собирается")
}

func (a *Assembler) paymentStatus(ctx context.Context, paymentID *uuid.UUID, authorization string) (string, error) {
	if paymentID == nil {
		return PaymentNone, nil
	}
	var payment paymentResponse
	status, err := a.payment.getJSON(ctx, "/api/v1/payments/"+paymentID.String(), authorization, &payment)
	if status == http.StatusNotFound {
		return PaymentNone, nil
	}
	if err != nil {
		return "", err
	}
	return payment.Status, nil
}

func IsNotFound(err error, service string) bool {
	var downstream *DownstreamError
	return errors.As(err, &downstream) && downstream.Service == service && downstream.Status == http.StatusNotFound
}
