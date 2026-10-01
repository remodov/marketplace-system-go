package http

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
)

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type AddressDTO struct {
	Country     string `json:"country"`
	City        string `json:"city"`
	Street      string `json:"street"`
	PostalCode  string `json:"postalCode"`
	PickupPoint string `json:"pickupPoint,omitempty"`
}

type OrderItemRequest struct {
	ProductID *uuid.UUID `json:"productId"`
	SellerID  *uuid.UUID `json:"sellerId"`
	Quantity  *int       `json:"quantity"`
}

type CreateOrderRequest struct {
	Items           []OrderItemRequest `json:"items"`
	ShippingAddress *AddressDTO        `json:"shippingAddress"`
}

type OrderItemDTO struct {
	ID        uuid.UUID       `json:"id"`
	ProductID uuid.UUID       `json:"productId"`
	SellerID  uuid.UUID       `json:"sellerId"`
	Quantity  int             `json:"quantity"`
	UnitPrice decimal.Decimal `json:"unitPrice"`
	LineTotal decimal.Decimal `json:"lineTotal"`
}

type OrderDTO struct {
	ID              uuid.UUID       `json:"id"`
	CustomerID      uuid.UUID       `json:"customerId"`
	SellerID        uuid.UUID       `json:"sellerId"`
	Status          string          `json:"status"`
	Items           []OrderItemDTO  `json:"items"`
	ShippingFee     decimal.Decimal `json:"shippingFee"`
	Total           decimal.Decimal `json:"total"`
	Currency        string          `json:"currency"`
	ShippingAddress AddressDTO      `json:"shippingAddress"`
	PaymentID       *uuid.UUID      `json:"paymentId,omitempty"`
	PaidAt          *time.Time      `json:"paidAt,omitempty"`
	ShippedAt       *time.Time      `json:"shippedAt,omitempty"`
	DeliveredAt     *time.Time      `json:"deliveredAt,omitempty"`
	ClosedAt        *time.Time      `json:"closedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type CancelOrderRequest struct {
	ReasonCode string `json:"reasonCode"`
	Comment    string `json:"comment"`
}

type ShipOrderRequest struct {
	TrackingNumber string `json:"trackingNumber"`
}

type PayOrderRequest struct {
	PaymentID *uuid.UUID `json:"paymentId"`
}

func toAddress(a AddressDTO) aggregate.Address {
	return aggregate.Address{Country: a.Country, City: a.City, Street: a.Street, PostalCode: a.PostalCode, PickupPoint: a.PickupPoint}
}

func toDTO(o *aggregate.Order) OrderDTO {
	items := make([]OrderItemDTO, 0, len(o.Items()))
	for _, item := range o.Items() {
		items = append(items, OrderItemDTO{
			ID: item.ID(), ProductID: item.ProductID(), SellerID: item.SellerID(), Quantity: item.Quantity(),
			UnitPrice: item.UnitPrice().Amount, LineTotal: item.LineTotal().Amount,
		})
	}
	address := o.ShippingAddress()
	total := o.Total()
	state := o.Lifecycle()
	return OrderDTO{
		ID: o.ID(), CustomerID: o.CustomerID(), SellerID: o.SellerID(), Status: string(o.Status()), Items: items,
		ShippingFee: o.ShippingFee().Amount, Total: total.Amount, Currency: total.Currency,
		ShippingAddress: AddressDTO{Country: address.Country, City: address.City, Street: address.Street, PostalCode: address.PostalCode, PickupPoint: address.PickupPoint},
		PaymentID:       state.PaymentID, PaidAt: utc(state.PaidAt), ShippedAt: utc(state.ShippedAt), DeliveredAt: utc(state.DeliveredAt), ClosedAt: utc(state.ClosedAt),
		CreatedAt: o.CreatedAt().UTC(), UpdatedAt: o.UpdatedAt().UTC(),
	}
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
