package product

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("товар не найден")
	ErrInvalid  = errors.New("недопустимое значение")
	ErrConflict = errors.New("товар изменили параллельно, повторите запрос")
)

type NotFoundError struct {
	ID uuid.UUID
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("Товар %s не найден", e.ID)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

type OutOfStockError struct {
	ID        uuid.UUID
	Requested int
	Available int
}

func (e *OutOfStockError) Error() string {
	return fmt.Sprintf("Товара %s не хватает: просят %d, на складе %d", e.ID, e.Requested, e.Available)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
