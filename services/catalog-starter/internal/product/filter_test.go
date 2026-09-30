package product_test

import (
	"net/http"
	"testing"
)

func TestCheaperThanFilter(t *testing.T) {
	prefix := unique("Фильтр")
	expensive := mustCreate(t, prefix+" дорогой", "2500.00", 1)
	cheap := mustCreate(t, prefix+" дешёвый", "1990.00", 1)
	boundary := mustCreate(t, prefix+" ровно по границе", "2000.00", 1)

	rec := call(t, http.MethodGet, "/products?maxPrice=2000", "")
	expectStatus(t, rec, http.StatusOK)
	found := list(t, rec)
	seen := map[string]bool{}
	last := 0.0
	for _, card := range found {
		price := card["price"].(float64)
		if price > 2000 {
			t.Fatalf("в выдаче товар дороже границы: %v", card)
		}
		if price < last {
			t.Fatalf("выдача не по возрастанию цены: %v после %v", price, last)
		}
		last = price
		seen[card["id"].(string)] = true
	}
	if !seen[cheap.ID().String()] {
		t.Fatalf("дешёвый товар не попал в выдачу")
	}
	if !seen[boundary.ID().String()] {
		t.Fatalf("товар ровно за maxPrice должен попадать в выдачу: граница включающая")
	}
	if seen[expensive.ID().String()] {
		t.Fatalf("дорогой товар попал в выдачу")
	}

	rec = call(t, http.MethodGet, "/products", "")
	expectStatus(t, rec, http.StatusOK)
	all := map[string]bool{}
	for _, card := range list(t, rec) {
		all[card["id"].(string)] = true
	}
	if !all[expensive.ID().String()] {
		t.Fatalf("без параметра каталог должен отдавать всё, дорогой товар пропал")
	}
}
