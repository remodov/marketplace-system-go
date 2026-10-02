package screen

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
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

	items := make([]Item, len(order.Items))
	var paymentStatus string
	eg, ctx := errgroup.WithContext(ctx)
	for i, line := range order.Items {
		eg.Go(func() error {
			var card productCard
			if _, err := a.catalog.getJSON(ctx, "/api/v1/products/"+line.ProductID.String(), authorization, &card); err != nil {
				return err
			}
			items[i] = Item{ProductID: line.ProductID, Title: card.Title, Quantity: line.Quantity, Price: card.Price}
			return nil
		})
	}
	eg.Go(func() error {
		status, err := a.paymentStatus(ctx, order.PaymentID, authorization)
		paymentStatus = status
		return err
	})
	if err := eg.Wait(); err != nil {
		return OrderScreen{}, err
	}
	return OrderScreen{OrderID: order.ID, Status: order.Status, Total: order.Total, PaymentStatus: paymentStatus, Items: items}, nil
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
