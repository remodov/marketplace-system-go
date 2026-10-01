package paymentsv1

import (
	"time"

	"github.com/google/uuid"
)

const (
	Topic = "marketplace.payments.v1"

	EventPaymentCompleted = "PaymentCompleted"
	EventPaymentFailed    = "PaymentFailed"
)

type PaymentCompletedPayload struct {
	PaymentID  uuid.UUID `json:"paymentId"`
	OrderID    uuid.UUID `json:"orderId"`
	Amount     string    `json:"amount"`
	Currency   string    `json:"currency"`
	OccurredAt time.Time `json:"occurredAt"`
}
