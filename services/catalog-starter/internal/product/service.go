package product

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Search(ctx context.Context, query string) ([]*Product, error) {
	if strings.TrimSpace(query) == "" {
		return s.store.All(ctx)
	}
	return s.store.ByTitle(ctx, strings.TrimSpace(query))
}

func (s *Service) CheaperThan(ctx context.Context, maxPrice decimal.Decimal) ([]*Product, error) {
	return s.store.Cheaper(ctx, maxPrice)
}

func (s *Service) Card(ctx context.Context, id uuid.UUID) (Card, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return Card{}, err
	}
	return CardOf(p), nil
}

func (s *Service) ByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	return s.store.ByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, title string, price decimal.Decimal, stock int) (*Product, error) {
	p, err := New(title, price, stock)
	if err != nil {
		return nil, err
	}
	if err := s.store.Insert(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) ChangePrice(ctx context.Context, id uuid.UUID, newPrice decimal.Decimal) (*Product, error) {
	return s.change(ctx, id, func(p *Product) error { return p.ChangePrice(newPrice) })
}

// TODO шаг 4: сценарий ApplyDiscount через change
func (s *Service) ApplyDiscount(ctx context.Context, id uuid.UUID, percent int) (*Product, error) {
	return s.store.ByID(ctx, id)
}

func (s *Service) ChangeStock(ctx context.Context, id uuid.UUID, delta int) (*Product, error) {
	return s.change(ctx, id, func(p *Product) error { return p.ChangeStock(delta) })
}

func (s *Service) change(ctx context.Context, id uuid.UUID, command func(*Product) error) (*Product, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := command(p); err != nil {
		return nil, err
	}
	if err := s.store.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) Reserve(ctx context.Context, id uuid.UUID, quantity int) (*Product, error) {
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := p.Reserve(quantity); err != nil {
		return nil, err
	}
	if err := s.store.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
