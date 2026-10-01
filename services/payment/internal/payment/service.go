package payment

import (
	"context"
	"database/sql"
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

// TODO шаг 11: заказ платят один раз - повторная авторизация того же заказа
// возвращает уже созданный платёж, а не списывает деньги второй раз.
func (s *Service) Authorize(ctx context.Context, orderID uuid.UUID, amount decimal.Decimal, currency string) (Payment, error) {
	now := s.clock.Now()
	payment := Payment{ID: uuid.New(), OrderID: orderID, Amount: amount.Round(2), Currency: currency, Status: StatusAuthorized, CreatedAt: now, UpdatedAt: now}
	err := s.inTx(ctx, func(tx *sql.Tx) error { return insert(ctx, tx, payment) })
	return payment, err
}

func (s *Service) Capture(ctx context.Context, id uuid.UUID) (Payment, error) {
	return s.moveTo(ctx, id, StatusCaptured)
}

// TODO шаг 11: повторный возврат это не второй возврат и не ошибка - сага может
// дойти до компенсации дважды, ответ тот же, деньги возвращаются один раз.
func (s *Service) Refund(ctx context.Context, id uuid.UUID) (Payment, error) {
	return s.moveTo(ctx, id, StatusRefunded)
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
