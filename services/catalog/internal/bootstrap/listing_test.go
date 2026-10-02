package bootstrap_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestListProducts_showsOnlyPublishedToEveryone(t *testing.T) {
	clearTables(t)
	seller := uuid.New()
	published := givenProduct(t, seller, "PUBLISHED", "1990.00")
	givenProduct(t, seller, "DRAFT", "100.00")
	givenProduct(t, seller, "HIDDEN", "200.00")

	rec := call(t, http.MethodGet, "/api/v1/products?sort=price,asc", "", "")

	expectStatus(t, rec, http.StatusOK)
	body := decode(t, rec)
	items, _ := body["items"].([]any)
	if len(items) != 1 || body["total"].(float64) != 1 {
		t.Fatalf("на витрине должен быть один опубликованный товар: %s", rec.Body.String())
	}
	first := items[0].(map[string]any)
	if first["id"] != published.String() || first["sellerId"] != seller.String() {
		t.Fatalf("карточка витрины должна нести идентификатор товара и продавца: %v", first)
	}
}
