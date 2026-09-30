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

// TODO шаг 3: сценарии ChangePrice и ChangeStock

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
