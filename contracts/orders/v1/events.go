package ordersv1

import (
	"time"

	"github.com/google/uuid"
)

const (
	Topic = "marketplace.orders.v1"

	HeaderEventID       = "event-id"
	HeaderEventType     = "event-type"
	HeaderEventVersion  = "event-version"
	HeaderAggregateType = "aggregate-type"
	HeaderAggregateID   = "aggregate-id"
	HeaderOccurredAt    = "occurred-at"

	EventOrderCreated    = "OrderCreated"
	EventOrderConfirmed  = "OrderConfirmed"
	EventOrderPaid       = "OrderPaid"
	EventOrderShipped    = "OrderShipped"
	EventOrderDelivered  = "OrderDelivered"
	EventOrderCompleted  = "OrderCompleted"
	EventOrderExpired    = "OrderExpired"
	EventOrderCancelled  = "OrderCancelled"
	EventDisputeOpened   = "DisputeOpened"
	EventDisputeResolved = "DisputeResolved"
)

type OrderEventBase struct {
	OrderID    uuid.UUID `json:"orderId"`
	CustomerID uuid.UUID `json:"customerId"`
	SellerID   uuid.UUID `json:"sellerId"`
	OccurredAt time.Time `json:"occurredAt"`
}

type OrderCreatedPayload struct {
	OrderEventBase
	TotalAmount string `json:"totalAmount"`
	Currency    string `json:"currency"`
	ItemsCount  int    `json:"itemsCount"`
}

type OrderConfirmedPayload struct {
	OrderEventBase
	TotalAmount string `json:"totalAmount"`
	Currency    string `json:"currency"`
}

type OrderPaidPayload struct {
	OrderEventBase
	TotalAmount string    `json:"totalAmount"`
	Currency    string    `json:"currency"`
	PaymentID   uuid.UUID `json:"paymentId"`
}

type OrderCancelledPayload struct {
	OrderEventBase
	PreviousStatus string     `json:"previousStatus"`
	Reason         string     `json:"reason,omitempty"`
	RefundID       *uuid.UUID `json:"refundId,omitempty"`
}

type DisputeOpenedPayload struct {
	OrderEventBase
	Reason string `json:"reason"`
}

type OrderShippedPayload struct {
	OrderEventBase
	TrackingNumber string `json:"trackingNumber"`
}

type OrderDeliveredPayload struct {
	OrderEventBase
}

type OrderExpiredPayload struct {
	OrderEventBase
}
