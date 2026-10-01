package aggregate

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
)

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusPublished Status = "PUBLISHED"
	StatusHidden    Status = "HIDDEN"
)

const SupportedCurrency = "RUB"

func ParseStatus(raw string) (Status, bool) {
	switch Status(raw) {
	case StatusDraft, StatusPublished, StatusHidden:
		return Status(raw), true
	}
	return "", false
}

type Product struct {
	id          uuid.UUID
	title       string
	description string
	price       decimal.Decimal
	currency    string
	sellerID    uuid.UUID
	status      Status
	createdAt   time.Time
	updatedAt   time.Time
}

func New(id, sellerID uuid.UUID, title, description string, price decimal.Decimal, currency string, now time.Time) (*Product, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, apperr.Invalid("VALIDATION_ERROR", "Название не может быть пустым")
	}
	if price.Sign() <= 0 {
		return nil, apperr.Invalid("INVALID_PRICE", "Цена должна быть больше нуля")
	}
	if currency != SupportedCurrency {
		return nil, apperr.Invalid("INVALID_CURRENCY", "Поддерживается только валюта RUB")
	}
	return &Product{
		id: id, sellerID: sellerID, title: title, description: strings.TrimSpace(description),
		price: price.Round(2), currency: currency, status: StatusDraft, createdAt: now, updatedAt: now,
	}, nil
}

func Restore(id uuid.UUID, title, description string, price decimal.Decimal, currency string, sellerID uuid.UUID, status Status, createdAt, updatedAt time.Time) *Product {
	return &Product{
		id: id, title: title, description: description, price: price, currency: currency,
		sellerID: sellerID, status: status, createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (p *Product) ID() uuid.UUID          { return p.id }
func (p *Product) Title() string          { return p.title }
func (p *Product) Description() string    { return p.description }
func (p *Product) Price() decimal.Decimal { return p.price }
func (p *Product) Currency() string       { return p.currency }
func (p *Product) SellerID() uuid.UUID    { return p.sellerID }
func (p *Product) Status() Status         { return p.status }
func (p *Product) CreatedAt() time.Time   { return p.createdAt }
func (p *Product) UpdatedAt() time.Time   { return p.updatedAt }

func (p *Product) OwnedBy(sellerID uuid.UUID) bool { return p.sellerID == sellerID }

func (p *Product) ChangePrice(newPrice decimal.Decimal, now time.Time) error {
	if newPrice.Sign() <= 0 {
		return apperr.Invalid("INVALID_PRICE", "Цена должна быть больше нуля, а не "+newPrice.String())
	}
	p.price = newPrice.Round(2)
	p.updatedAt = now
	return nil
}

func (p *Product) Publish(now time.Time) error {
	if p.status != StatusDraft && p.status != StatusHidden {
		return transitionError(p.status, StatusPublished)
	}
	p.status = StatusPublished
	p.updatedAt = now
	return nil
}

func (p *Product) Hide(now time.Time) error {
	if p.status != StatusPublished {
		return transitionError(p.status, StatusHidden)
	}
	p.status = StatusHidden
	p.updatedAt = now
	return nil
}

func transitionError(from, to Status) error {
	return apperr.Conflict("INVALID_STATE_TRANSITION", "Переход "+string(from)+" -> "+string(to)+" не разрешён")
}
