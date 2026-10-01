package payment

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Status string

const (
	StatusAuthorized Status = "AUTHORIZED"
	StatusCaptured   Status = "CAPTURED"
	StatusRefunded   Status = "REFUNDED"
	StatusFailed     Status = "FAILED"
)

func (s Status) CanMoveTo(next Status) bool {
	switch s {
	case StatusAuthorized:
		return next == StatusCaptured || next == StatusRefunded || next == StatusFailed
	case StatusCaptured:
		return next == StatusRefunded
	default:
		return false
	}
}

type InvalidTransitionError struct {
	PaymentID uuid.UUID
	From, To  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("платёж %s: переход %s -> %s запрещён", e.PaymentID, e.From, e.To)
}

type Payment struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	Amount    decimal.Decimal
	Currency  string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p Payment) MoveTo(next Status, now time.Time) (Payment, error) {
	if !p.Status.CanMoveTo(next) {
		return Payment{}, &InvalidTransitionError{PaymentID: p.ID, From: p.Status, To: next}
	}
	p.Status = next
	p.UpdatedAt = now
	return p, nil
}
