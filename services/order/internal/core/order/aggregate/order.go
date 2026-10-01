package aggregate

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
)

const (
	Currency    = "RUB"
	MaxQuantity = 999
)

var MinConfirmAmount = decimal.NewFromInt(100)

type CancellationReason struct {
	Code    string
	Comment string
}

func NewCancellationReason(code, comment string) (CancellationReason, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return CancellationReason{}, apperr.Invalid("VALIDATION_ERROR", "Нужен код причины отмены")
	}
	if len([]rune(comment)) > 500 {
		return CancellationReason{}, apperr.Invalid("VALIDATION_ERROR", "Комментарий к отмене не длиннее 500 символов")
	}
	return CancellationReason{Code: code, Comment: comment}, nil
}

type LifecycleState struct {
	PaymentID   *uuid.UUID
	PaidAt      *time.Time
	ShippedAt   *time.Time
	DeliveredAt *time.Time
	ClosedAt    *time.Time
}

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
	lifecycle   LifecycleState
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
	order.register(OrderCreated{eventBase: order.base(now), Total: order.Total(), Items: snapshotsOf(items)})
	return order, nil
}

func Restore(id, customerID, sellerID uuid.UUID, status Status, items []Item, shippingFee Money, address Address, createdAt, updatedAt time.Time, lifecycle LifecycleState) *Order {
	return &Order{
		id: id, customerID: customerID, sellerID: sellerID, status: status,
		items: slices.Clone(items), shippingFee: shippingFee, address: address,
		createdAt: createdAt, updatedAt: updatedAt, lifecycle: lifecycle,
	}
}

func (o *Order) ID() uuid.UUID             { return o.id }
func (o *Order) CustomerID() uuid.UUID     { return o.customerID }
func (o *Order) SellerID() uuid.UUID       { return o.sellerID }
func (o *Order) Status() Status            { return o.status }
func (o *Order) Items() []Item             { return slices.Clone(o.items) }
func (o *Order) ShippingFee() Money        { return o.shippingFee }
func (o *Order) ShippingAddress() Address  { return o.address }
func (o *Order) CreatedAt() time.Time      { return o.createdAt }
func (o *Order) UpdatedAt() time.Time      { return o.updatedAt }
func (o *Order) Lifecycle() LifecycleState { return o.lifecycle }

func (o *Order) Total() Money {
	total := RUB(decimal.Zero)
	for _, item := range o.items {
		total = total.Add(item.LineTotal())
	}
	return total.Add(o.shippingFee)
}

func (o *Order) OwnedBy(customerID uuid.UUID) bool { return o.customerID == customerID }
func (o *Order) SoldBy(sellerID uuid.UUID) bool    { return o.sellerID == sellerID }

func (o *Order) Confirm(now time.Time) error {
	if err := o.require(StatusDraft, "подтвердить"); err != nil {
		return err
	}
	if len(o.items) == 0 {
		return apperr.Invalid("EMPTY_ORDER", "В заказе нет ни одной позиции")
	}
	total := o.Total()
	if total.Amount.LessThan(MinConfirmAmount) {
		return apperr.Invalid("ORDER_BELOW_MINIMUM", "Сумма заказа "+total.Amount.StringFixed(2)+" меньше минимальной "+MinConfirmAmount.StringFixed(2))
	}
	o.moveTo(StatusPendingPayment, now)
	o.register(OrderConfirmed{eventBase: o.base(now), Total: total})
	return nil
}

func (o *Order) MarkPaid(paymentID uuid.UUID, now time.Time) error {
	if err := o.require(StatusPendingPayment, "оплатить"); err != nil {
		return err
	}
	o.moveTo(StatusPaid, now)
	o.lifecycle.PaymentID = &paymentID
	o.lifecycle.PaidAt = &now
	o.register(OrderPaid{eventBase: o.base(now), PaymentID: paymentID, Total: o.Total()})
	return nil
}

func (o *Order) Cancel(reason CancellationReason, now time.Time) error {
	if o.status != StatusDraft && o.status != StatusPendingPayment {
		return o.invalidState("отменить без возврата")
	}
	previous := o.status
	o.moveTo(StatusCancelled, now)
	o.lifecycle.ClosedAt = &now
	o.register(OrderCancelled{eventBase: o.base(now), PreviousStatus: previous, Reason: reason})
	return nil
}

func (o *Order) CancelAfterPayment(reason CancellationReason, refundID uuid.UUID, now time.Time) error {
	if err := o.require(StatusPaid, "отменить с возвратом"); err != nil {
		return err
	}
	previous := o.status
	o.moveTo(StatusCancelled, now)
	o.lifecycle.ClosedAt = &now
	o.register(OrderCancelled{eventBase: o.base(now), PreviousStatus: previous, Reason: reason, RefundID: &refundID})
	return nil
}

func (o *Order) Expire(now time.Time) error {
	if err := o.require(StatusPendingPayment, "закрыть по таймауту"); err != nil {
		return err
	}
	o.moveTo(StatusExpired, now)
	o.lifecycle.ClosedAt = &now
	o.register(OrderExpired{eventBase: o.base(now)})
	return nil
}

func (o *Order) MarkShipped(trackingNumber string, now time.Time) error {
	if strings.TrimSpace(trackingNumber) == "" {
		return apperr.Invalid("VALIDATION_ERROR", "Нужен трек-номер отправления")
	}
	if err := o.require(StatusPaid, "передать в доставку"); err != nil {
		return err
	}
	o.moveTo(StatusShipped, now)
	o.lifecycle.ShippedAt = &now
	o.register(OrderShipped{eventBase: o.base(now), TrackingNumber: trackingNumber})
	return nil
}

func (o *Order) ConfirmDelivery(now time.Time) error {
	if err := o.require(StatusShipped, "подтвердить получение"); err != nil {
		return err
	}
	o.moveTo(StatusDelivered, now)
	o.lifecycle.DeliveredAt = &now
	o.register(OrderDelivered{eventBase: o.base(now)})
	return nil
}

func (o *Order) require(expected Status, action string) error {
	if o.status != expected {
		return o.invalidState(action)
	}
	return nil
}

func (o *Order) invalidState(action string) error {
	return apperr.Conflict("ORDER_INVALID_STATE", fmt.Sprintf("Заказ в статусе %s нельзя %s", o.status, action))
}

func (o *Order) moveTo(next Status, now time.Time) {
	o.status = next
	o.updatedAt = now
}

func (o *Order) base(now time.Time) eventBase {
	return eventBase{OrderID: o.id, CustomerID: o.customerID, SellerID: o.sellerID, At: now}
}

func (o *Order) register(event Event) {
	o.events = append(o.events, event)
}

func (o *Order) PullEvents() []Event {
	events := o.events
	o.events = nil
	return events
}
