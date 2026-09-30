package product

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const MaxDiscountPercent = 50

type Product struct {
	id       uuid.UUID
	title    string
	price    decimal.Decimal
	stock    int
	reserved int
	version  int64
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

func Restore(id uuid.UUID, title string, price decimal.Decimal, stock, reserved int, version int64) *Product {
	return &Product{id: id, title: title, price: price, stock: stock, reserved: reserved, version: version}
}

func (p *Product) ID() uuid.UUID          { return p.id }
func (p *Product) Title() string          { return p.title }
func (p *Product) Price() decimal.Decimal { return p.price }
func (p *Product) Stock() int             { return p.stock }
func (p *Product) Version() int64         { return p.version }

func (p *Product) Reserved() int  { return p.reserved }
func (p *Product) Available() int { return p.stock - p.reserved }

func (p *Product) ChangePrice(newPrice decimal.Decimal) error {
	if newPrice.Sign() <= 0 {
		return invalid("цена должна быть больше нуля")
	}
	p.price = newPrice
	return nil
}

func (p *Product) ApplyDiscount(percent int) error {
	if percent < 1 || percent > MaxDiscountPercent {
		return invalid("скидка допустима от 1 до %d процентов, а не %d", MaxDiscountPercent, percent)
	}
	multiplier := decimal.NewFromInt(int64(100 - percent)).Div(decimal.NewFromInt(100))
	p.price = p.price.Mul(multiplier).Round(2)
	return nil
}

func (p *Product) ChangeStock(delta int) error {
	if delta == 0 {
		return invalid("изменение остатка не может быть нулевым")
	}
	if p.stock+delta < p.reserved {
		return &OutOfStockError{ID: p.id, Requested: -delta, Available: p.Available()}
	}
	p.stock += delta
	return nil
}

func (p *Product) Reserve(quantity int) error {
	if quantity <= 0 {
		return invalid("количество должно быть больше нуля")
	}
	if quantity > p.Available() {
		return &OutOfStockError{ID: p.id, Requested: quantity, Available: p.Available()}
	}
	p.reserved += quantity
	return nil
}
