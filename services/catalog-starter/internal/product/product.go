package product

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Product struct {
	id      uuid.UUID
	title   string
	price   decimal.Decimal
	stock   int
	version int64
}

func New(title string, price decimal.Decimal, stock int) (*Product, error) {
	if strings.TrimSpace(title) == "" {
		return nil, invalid("название не может быть пустым")
	}
	if price.Sign() <= 0 {
		return nil, invalid("цена должна быть больше нуля")
	}
	if stock < 0 {
		return nil, invalid("остаток не может быть отрицательным")
	}
	return &Product{id: uuid.New(), title: strings.TrimSpace(title), price: price, stock: stock}, nil
}

func Restore(id uuid.UUID, title string, price decimal.Decimal, stock int, version int64) *Product {
	return &Product{id: id, title: title, price: price, stock: stock, version: version}
}

func (p *Product) ID() uuid.UUID          { return p.id }
func (p *Product) Title() string          { return p.title }
func (p *Product) Price() decimal.Decimal { return p.price }
func (p *Product) Stock() int             { return p.stock }
func (p *Product) Version() int64         { return p.version }

// TODO шаг 3: команды ChangePrice и ChangeStock с правилами отказа

func (p *Product) Reserve(quantity int) error {
	if quantity <= 0 {
		return invalid("количество должно быть больше нуля")
	}
	if quantity > p.stock {
		return &OutOfStockError{ID: p.id, Requested: quantity, Available: p.stock}
	}
	p.stock -= quantity
	return nil
}
