package aggregate

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
)

const (
	Currency    = "RUB"
	MaxQuantity = 999
)

type Status string

const (
	StatusDraft          Status = "DRAFT"
	StatusPendingPayment Status = "PENDING_PAYMENT"
	StatusPaid           Status = "PAID"
	StatusShipped        Status = "SHIPPED"
	StatusDelivered      Status = "DELIVERED"
	StatusCompleted      Status = "COMPLETED"
	StatusExpired        Status = "EXPIRED"
	StatusCancelled      Status = "CANCELLED"
	StatusDispute        Status = "DISPUTE"
	StatusRefunded       Status = "REFUNDED"
)

type Money struct {
	Amount   decimal.Decimal
	Currency string
}

func RUB(amount decimal.Decimal) Money {
	return Money{Amount: amount.Round(2), Currency: Currency}
}

func (m Money) Add(other Money) Money {
	return Money{Amount: m.Amount.Add(other.Amount), Currency: m.Currency}
}

func (m Money) Times(n int) Money {
	return Money{Amount: m.Amount.Mul(decimal.NewFromInt(int64(n))), Currency: m.Currency}
}

type Address struct {
	Country     string
	City        string
	Street      string
	PostalCode  string
	PickupPoint string
}

type Item struct {
	id        uuid.UUID
	productID uuid.UUID
	sellerID  uuid.UUID
	quantity  int
	unitPrice Money
}

func NewItem(id, productID, sellerID uuid.UUID, quantity int, unitPrice Money) (Item, error) {
	if quantity < 1 || quantity > MaxQuantity {
		return Item{}, apperr.Invalid("VALIDATION_ERROR", fmt.Sprintf("Количество должно быть от 1 до %d", MaxQuantity))
	}
	if unitPrice.Amount.Sign() < 0 || unitPrice.Currency != Currency {
		return Item{}, apperr.Invalid("INVALID_PRICE", "Цена позиции должна быть неотрицательной в рублях")
	}
	return Item{id: id, productID: productID, sellerID: sellerID, quantity: quantity, unitPrice: RUB(unitPrice.Amount)}, nil
}

func RestoreItem(id, productID, sellerID uuid.UUID, quantity int, unitPrice Money) Item {
	return Item{id: id, productID: productID, sellerID: sellerID, quantity: quantity, unitPrice: unitPrice}
}

func (i Item) ID() uuid.UUID        { return i.id }
func (i Item) ProductID() uuid.UUID { return i.productID }
func (i Item) SellerID() uuid.UUID  { return i.sellerID }
func (i Item) Quantity() int        { return i.quantity }
func (i Item) UnitPrice() Money     { return i.unitPrice }
func (i Item) LineTotal() Money     { return i.unitPrice.Times(i.quantity) }

type Order struct {
	id          uuid.UUID
	customerID  uuid.UUID
	sellerID    uuid.UUID
	status      Status
	items       []Item
	shippingFee Money
	address     Address
	createdAt   time.Time
	updatedAt   time.Time
	events      []Event
}

func New(id, customerID uuid.UUID, items []Item, address Address, now time.Time) (*Order, error) {
	if len(items) == 0 {
		return nil, apperr.Invalid("EMPTY_ORDER", "В заказе нет ни одной позиции")
	}
	seller := items[0].sellerID
	seen := make(map[uuid.UUID]bool, len(items))
	for _, item := range items {
		if item.sellerID != seller {
			return nil, apperr.Invalid("MULTI_SELLER_NOT_SUPPORTED", "В одном заказе могут быть товары только одного продавца")
		}
		if seen[item.productID] {
			return nil, apperr.Invalid("VALIDATION_ERROR", "Товар "+item.productID.String()+" повторяется в позициях заказа")
		}
		seen[item.productID] = true
	}
	order := &Order{
		id: id, customerID: customerID, sellerID: seller, status: StatusDraft,
		items: slices.Clone(items), shippingFee: RUB(decimal.Zero), address: address,
		createdAt: now, updatedAt: now,
	}
	order.events = append(order.events, OrderCreated{
		OrderID: id, CustomerID: customerID, SellerID: seller, Total: order.Total(), Items: snapshotsOf(items), At: now,
	})
	return order, nil
}

func Restore(id, customerID, sellerID uuid.UUID, status Status, items []Item, shippingFee Money, address Address, createdAt, updatedAt time.Time) *Order {
	return &Order{
		id: id, customerID: customerID, sellerID: sellerID, status: status,
		items: slices.Clone(items), shippingFee: shippingFee, address: address,
		createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (o *Order) ID() uuid.UUID            { return o.id }
func (o *Order) CustomerID() uuid.UUID    { return o.customerID }
func (o *Order) SellerID() uuid.UUID      { return o.sellerID }
func (o *Order) Status() Status           { return o.status }
func (o *Order) Items() []Item            { return slices.Clone(o.items) }
func (o *Order) ShippingFee() Money       { return o.shippingFee }
func (o *Order) ShippingAddress() Address { return o.address }
func (o *Order) CreatedAt() time.Time     { return o.createdAt }
func (o *Order) UpdatedAt() time.Time     { return o.updatedAt }

func (o *Order) Total() Money {
	total := RUB(decimal.Zero)
	for _, item := range o.items {
		total = total.Add(item.LineTotal())
	}
	return total.Add(o.shippingFee)
}

func (o *Order) OwnedBy(customerID uuid.UUID) bool { return o.customerID == customerID }

func (o *Order) PullEvents() []Event {
	events := o.events
	o.events = nil
	return events
}
