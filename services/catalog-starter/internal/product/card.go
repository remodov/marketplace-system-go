package product

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Card struct {
	ID    uuid.UUID       `json:"id"`
	Title string          `json:"title"`
	Price decimal.Decimal `json:"price"`
	Stock int             `json:"stock"`
}

func CardOf(p *Product) Card {
	return Card{ID: p.ID(), Title: p.Title(), Price: p.Price(), Stock: p.Stock()}
}
