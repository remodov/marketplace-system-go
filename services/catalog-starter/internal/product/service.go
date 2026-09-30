package product

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/cache"
)

type Service struct {
	store Store
	cache cache.Cache
}

func NewService(store Store, cache cache.Cache) *Service {
	return &Service{store: store, cache: cache}
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
	var card Card
	hit, err := s.cache.Get(ctx, cardKey(id), &card)
	if err != nil {
		slog.Warn("кэш карточек недоступен, читаем из базы", "err", err)
	}
	if hit {
		return card, nil
	}
	p, err := s.store.ByID(ctx, id)
	if err != nil {
		return Card{}, err
	}
	card = CardOf(p)
	if err := s.cache.Set(ctx, cardKey(id), card); err != nil {
		slog.Warn("карточка не попала в кэш", "err", err)
	}
	return card, nil
}

func cardKey(id uuid.UUID) string {
	return "product-card:" + id.String()
}

func (s *Service) forget(ctx context.Context, id uuid.UUID) {
	if err := s.cache.Delete(ctx, cardKey(id)); err != nil {
		slog.Warn("карточка не сброшена из кэша", "id", id, "err", err)
	}
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

func (s *Service) ApplyDiscount(ctx context.Context, id uuid.UUID, percent int) (*Product, error) {
	return s.change(ctx, id, func(p *Product) error { return p.ApplyDiscount(percent) })
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
	s.forget(ctx, id)
	return p, nil
}

func (s *Service) Reserve(ctx context.Context, id uuid.UUID, quantity int) (*Product, error) {
	var reserved *Product
	err := s.store.WithTx(ctx, func(tx Store) error {
		p, err := tx.ByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := p.Reserve(quantity); err != nil {
			return err
		}
		if err := tx.Update(ctx, p); err != nil {
			return err
		}
		reserved = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.forget(ctx, id)
	return reserved, nil
}
