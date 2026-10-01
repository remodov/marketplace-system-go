package payment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Clock interface {
	Now() time.Time
}

type Service struct {
	db    *sql.DB
	clock Clock
}

func NewService(db *sql.DB, clock Clock) *Service {
	return &Service{db: db, clock: clock}
}

func (s *Service) Authorize(ctx context.Context, orderID uuid.UUID, amount decimal.Decimal, currency string) (Payment, error) {
	var result Payment
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		existing, err := findByOrderID(ctx, tx, orderID)
		if err == nil {
			result = existing
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		now := s.clock.Now()
		result = Payment{ID: uuid.New(), OrderID: orderID, Amount: amount.Round(2), Currency: currency, Status: StatusAuthorized, CreatedAt: now, UpdatedAt: now}
		return insert(ctx, tx, result)
	})
	return result, err
}

func (s *Service) Capture(ctx context.Context, id uuid.UUID) (Payment, error) {
	return s.moveTo(ctx, id, StatusCaptured)
}

func (s *Service) Refund(ctx context.Context, id uuid.UUID) (Payment, error) {
	var result Payment
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		current, err := findByID(ctx, tx, id)
		if err != nil {
			return err
		}
		if current.Status == StatusRefunded {
			result = current
			return nil
		}
		moved, err := current.MoveTo(StatusRefunded, s.clock.Now())
		if err != nil {
			return err
		}
		result = moved
		return updateStatus(ctx, tx, moved)
	})
	return result, err
}

func (s *Service) ByID(ctx context.Context, id uuid.UUID) (Payment, error) {
	return findByID(ctx, s.db, id)
}

func (s *Service) moveTo(ctx context.Context, id uuid.UUID, next Status) (Payment, error) {
	var result Payment
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		current, err := findByID(ctx, tx, id)
		if err != nil {
			return err
		}
		moved, err := current.MoveTo(next, s.clock.Now())
		if err != nil {
			return err
		}
		result = moved
		return updateStatus(ctx, tx, moved)
	})
	return result, err
}

func (s *Service) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
