package product_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/product"
)

func mustNew(t *testing.T, price string, stock int) *product.Product {
	t.Helper()
	p, err := product.New("Товар", decimal.RequireFromString(price), stock)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPriceMustBePositive(t *testing.T) {
	for _, price := range []string{"0", "-1"} {
		if _, err := product.New("Товар", decimal.RequireFromString(price), 1); !errors.Is(err, product.ErrInvalid) {
			t.Fatalf("цена %s должна отвергаться, получили %v", price, err)
		}
	}
}

func TestDiscountOutsideRangeIsRejectedAndKeepsPrice(t *testing.T) {
	p := mustNew(t, "1000.00", 1)
	for _, percent := range []int{0, -5, product.MaxDiscountPercent + 1, 100} {
		if err := p.ApplyDiscount(percent); !errors.Is(err, product.ErrInvalid) {
			t.Fatalf("скидка %d%% должна отвергаться, получили %v", percent, err)
		}
		if !p.Price().Equal(decimal.RequireFromString("1000.00")) {
			t.Fatalf("неудачная скидка изменила цену: %s", p.Price())
		}
	}
}

func TestDiscountAtTheLimitIsAllowed(t *testing.T) {
	p := mustNew(t, "1000.00", 1)
	if err := p.ApplyDiscount(product.MaxDiscountPercent); err != nil {
		t.Fatal(err)
	}
	if p.Price().String() != "500" {
		t.Fatalf("половина от тысячи: %s", p.Price())
	}
}

func TestDiscountIsRoundedToKopecks(t *testing.T) {
	p := mustNew(t, "999.99", 1)
	if err := p.ApplyDiscount(33); err != nil {
		t.Fatal(err)
	}
	if p.Price().String() != "669.99" {
		t.Fatalf("33%% от 999.99 это 669.9933, ждём 669.99, получили %s", p.Price())
	}
}

func TestZeroStockChangeIsRejected(t *testing.T) {
	p := mustNew(t, "10.00", 3)
	if err := p.ChangeStock(0); !errors.Is(err, product.ErrInvalid) {
		t.Fatalf("нулевое изменение остатка: %v", err)
	}
}

func TestWriteOffBelowZeroIsRejected(t *testing.T) {
	p := mustNew(t, "10.00", 3)
	var outOfStock *product.OutOfStockError
	if err := p.ChangeStock(-4); !errors.As(err, &outOfStock) {
		t.Fatalf("списание ниже нуля: %v", err)
	}
	if p.Stock() != 3 {
		t.Fatalf("остаток после отказа: %d", p.Stock())
	}
}

func TestReserveMoreThanAvailableIsRejected(t *testing.T) {
	p := mustNew(t, "10.00", 3)
	var outOfStock *product.OutOfStockError
	if err := p.Reserve(4); !errors.As(err, &outOfStock) {
		t.Fatalf("резерв сверх остатка: %v", err)
	}
	if outOfStock.Requested != 4 || outOfStock.Available != 3 {
		t.Fatalf("в ошибке не те числа: %+v", outOfStock)
	}
}

func TestStateIsChangedOnlyThroughMethods(t *testing.T) {
	typ := reflect.TypeOf(product.Product{})
	for i := 0; i < typ.NumField(); i++ {
		if field := typ.Field(i); field.IsExported() {
			t.Fatalf("поле %s экспортировано: правило можно обойти, поменяв его напрямую", field.Name)
		}
	}
}
