package bootstrap_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestChangePrice_whenOwner_returns200AndUpdatesDatabase(t *testing.T) {
	clearTables(t)
	seller := uuid.New()
	id := givenProduct(t, seller, "PUBLISHED", "89990.00")

	rec := call(t, http.MethodPatch, "/api/v1/products/"+id.String()+"/price", sellerToken(seller), `{"price": 79990.00}`)

	expectStatus(t, rec, http.StatusOK)
	if got := decode(t, rec)["price"]; got != 79990.0 {
		t.Fatalf("цена в ответе %v", got)
	}
	if got := priceInDB(t, id); got != "79990.00" {
		t.Fatalf("цена в базе %s", got)
	}
}

func TestChangePrice_whenNotPositive_returns400(t *testing.T) {
	clearTables(t)
	seller := uuid.New()
	id := givenProduct(t, seller, "PUBLISHED", "89990.00")

	rec := call(t, http.MethodPatch, "/api/v1/products/"+id.String()+"/price", sellerToken(seller), `{"price": 0}`)

	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "VALIDATION_ERROR")
	if got := priceInDB(t, id); got != "89990.00" {
		t.Fatalf("цена в базе изменилась: %s", got)
	}
}

func TestChangePrice_whenForeignProduct_returns404(t *testing.T) {
	clearTables(t)
	seller, other := uuid.New(), uuid.New()
	id := givenProduct(t, other, "PUBLISHED", "89990.00")

	rec := call(t, http.MethodPatch, "/api/v1/products/"+id.String()+"/price", sellerToken(seller), `{"price": 79990.00}`)

	expectStatus(t, rec, http.StatusNotFound)
	expectCode(t, rec, "OWN_PRODUCT_REQUIRED")
	if got := priceInDB(t, id); got != "89990.00" {
		t.Fatalf("цена в базе изменилась: %s", got)
	}
}

func TestChangePrice_whenUnknownProduct_returns404(t *testing.T) {
	clearTables(t)

	rec := call(t, http.MethodPatch, "/api/v1/products/"+uuid.NewString()+"/price", sellerToken(uuid.New()), `{"price": 79990.00}`)

	expectStatus(t, rec, http.StatusNotFound)
	expectCode(t, rec, "PRODUCT_NOT_FOUND")
}

func TestChangePrice_whenAdmin_returns200AndWritesAudit(t *testing.T) {
	clearTables(t)
	admin, other := uuid.New(), uuid.New()
	id := givenProduct(t, other, "PUBLISHED", "89990.00")

	rec := call(t, http.MethodPatch, "/api/v1/products/"+id.String()+"/price", adminToken(admin), `{"price": 1000.00}`)

	expectStatus(t, rec, http.StatusOK)
	if got := decode(t, rec)["price"]; got != 1000.0 {
		t.Fatalf("цена в ответе %v", got)
	}
	if !slices.Contains(auditActions(t), "PRODUCT_PRICE_CHANGED") {
		t.Fatalf("в журнале нет PRODUCT_PRICE_CHANGED: %v", auditActions(t))
	}
}

func TestChangePrice_whenAnonymous_isRejected(t *testing.T) {
	clearTables(t)
	id := givenProduct(t, uuid.New(), "PUBLISHED", "89990.00")

	rec := call(t, http.MethodPatch, "/api/v1/products/"+id.String()+"/price", "", `{"price": 1.00}`)

	expectStatus(t, rec, http.StatusUnauthorized)
	expectCode(t, rec, "TOKEN_MISSING")
	if got := priceInDB(t, id); got != "89990.00" {
		t.Fatalf("цена в базе изменилась: %s", got)
	}
}
