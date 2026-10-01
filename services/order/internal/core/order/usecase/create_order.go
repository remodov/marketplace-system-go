package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/security"
)

type OrderLine struct {
	ProductID uuid.UUID
	SellerID  uuid.UUID
	Quantity  int
}

type CreateOrder struct {
	Customer        security.Principal
	Lines           []OrderLine
	ShippingAddress aggregate.Address
	IdempotencyKey  string
	RequestHash     string
}

type CreateOrderResult struct {
	Order   *aggregate.Order
	Created bool
}

type CreateOrderHandler struct {
	orders  out.OrderRepository
	catalog out.CatalogGateway
	keys    out.IdempotencyKeys
	clock   out.Clock
	ids     out.IDGenerator
	uow     out.UnitOfWork
}

func NewCreateOrderHandler(orders out.OrderRepository, catalog out.CatalogGateway, keys out.IdempotencyKeys, clock out.Clock, ids out.IDGenerator, uow out.UnitOfWork) *CreateOrderHandler {
	return &CreateOrderHandler{orders: orders, catalog: catalog, keys: keys, clock: clock, ids: ids, uow: uow}
}

var errKeyTaken = errors.New("ключ идемпотентности занят другим запросом")

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder) (CreateOrderResult, error) {
	if len(cmd.Lines) == 0 {
		return CreateOrderResult{}, apperr.Invalid("EMPTY_ORDER", "В заказе нет ни одной позиции")
	}
	if err := requireSingleSeller(cmd.Lines); err != nil {
		return CreateOrderResult{}, err
	}
	if existing, found, err := h.keys.Find(ctx, cmd.IdempotencyKey, cmd.RequestHash); err != nil || found {
		return h.replay(ctx, existing, err)
	}
	order, err := h.build(ctx, cmd)
	if err != nil {
		return CreateOrderResult{}, err
	}
	err = h.uow.Within(ctx, func(ctx context.Context) error {
		if err := h.orders.Insert(ctx, order); err != nil {
			return err
		}
		claimed, err := h.keys.Claim(ctx, cmd.IdempotencyKey, cmd.RequestHash, order.ID(), order.CreatedAt())
		if err != nil {
			return err
		}
		if !claimed {
			return errKeyTaken
		}
		return nil
	})
	if errors.Is(err, errKeyTaken) {
		existing, _, err := h.keys.Find(ctx, cmd.IdempotencyKey, cmd.RequestHash)
		return h.replay(ctx, existing, err)
	}
	if err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: order, Created: true}, nil
}

func (h *CreateOrderHandler) replay(ctx context.Context, orderID uuid.UUID, err error) (CreateOrderResult, error) {
	if err != nil {
		return CreateOrderResult{}, err
	}
	order, err := h.orders.ByID(ctx, orderID)
	if err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: order, Created: false}, nil
}

func (h *CreateOrderHandler) build(ctx context.Context, cmd CreateOrder) (*aggregate.Order, error) {
	prices, err := h.catalog.Prices(ctx, productIDs(cmd.Lines))
	if err != nil {
		return nil, err
	}
	items := make([]aggregate.Item, 0, len(cmd.Lines))
	for _, line := range cmd.Lines {
		price, ok := prices[line.ProductID]
		if !ok {
			return nil, apperr.NotFound("PRODUCT_NOT_FOUND", "Товар "+line.ProductID.String()+" не найден в каталоге")
		}
		item, err := aggregate.NewItem(h.ids.NewID(), line.ProductID, line.SellerID, line.Quantity, price)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return aggregate.New(h.ids.NewID(), cmd.Customer.Sub, items, cmd.ShippingAddress, h.clock.Now())
}

func requireSingleSeller(lines []OrderLine) error {
	for _, line := range lines[1:] {
		if line.SellerID != lines[0].SellerID {
			return apperr.Invalid("MULTI_SELLER_NOT_SUPPORTED", "В одном заказе могут быть товары только одного продавца")
		}
	}
	return nil
}

func productIDs(lines []OrderLine) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(lines))
	ids := make([]uuid.UUID, 0, len(lines))
	for _, line := range lines {
		if !seen[line.ProductID] {
			seen[line.ProductID] = true
			ids = append(ids, line.ProductID)
		}
	}
	return ids
}
