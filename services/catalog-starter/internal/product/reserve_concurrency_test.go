package product_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

const (
	buyers = 100
	stock  = 10
)

func TestHundredBuyersSellExactlyTheStock(t *testing.T) {
	ctx := context.Background()
	p := mustCreate(t, unique("Билет на распродажу"), "100.00", stock)

	var wg sync.WaitGroup
	var sold atomic.Int32
	for i := 0; i < buyers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.Reserve(ctx, p.ID(), 1); err == nil {
				sold.Add(1)
			}
		}()
	}
	wg.Wait()

	after, err := service.ByID(ctx, p.ID())
	if err != nil {
		t.Fatal(err)
	}
	if got := sold.Load(); got != stock {
		t.Fatalf("успешных резервов %d, а товара было %d", got, stock)
	}
	if after.Reserved() != stock || after.Available() != 0 {
		t.Fatalf("после распродажи reserved=%d available=%d", after.Reserved(), after.Available())
	}
	if after.Stock() != stock {
		t.Fatalf("остаток на складе резерв не трогает, а он стал %d", after.Stock())
	}
}
