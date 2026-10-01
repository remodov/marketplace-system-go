package usecase

import (
	"context"

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

// TODO шаг 9: до работы спросить у keys прежний заказ по ключу и хешу (конфликт
// хеша уходит наружу как есть), после сборки заказа записать его и занять ключ в
// одной транзакции; если ключ занять не удалось, вернуть чужой заказ с Created=false.
func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder) (CreateOrderResult, error) {
	if len(cmd.Lines) == 0 {
		return CreateOrderResult{}, apperr.Invalid("EMPTY_ORDER", "В заказе нет ни одной позиции")
	}
	if err := requireSingleSeller(cmd.Lines); err != nil {
		return CreateOrderResult{}, err
	}
	order, err := h.build(ctx, cmd)
	if err != nil {
		return CreateOrderResult{}, err
	}
	if err := h.uow.Within(ctx, func(ctx context.Context) error { return h.orders.Insert(ctx, order) }); err != nil {
		return CreateOrderResult{}, err
	}
	return CreateOrderResult{Order: order, Created: true}, nil
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
