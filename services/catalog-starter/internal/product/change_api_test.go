package product_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPriceIsChanged(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/price", `{"price":1490.00}`)
	expectStatus(t, rec, http.StatusOK)
	if got := body(t, rec)["price"]; got != float64(1490) {
		t.Fatalf("цена в ответе: %v", got)
	}
	rec = call(t, http.MethodGet, "/products/"+p.ID().String(), "")
	if got := body(t, rec)["price"]; got != float64(1490) {
		t.Fatalf("цена после перечитывания: %v", got)
	}
}

func TestNegativePriceIsRejectedWithFieldName(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/price", `{"price":-1}`)
	expectStatus(t, rec, http.StatusBadRequest)
	errs, _ := body(t, rec)["errors"].(map[string]any)
	if errs["price"] == nil {
		t.Fatalf("тело 400 обязано назвать поле price: %s", rec.Body.String())
	}
}

func TestPriceOfUnknownProductIsNotFound(t *testing.T) {
	missing := uuid.NewString()
	rec := call(t, http.MethodPatch, "/products/"+missing+"/price", `{"price":100.00}`)
	expectStatus(t, rec, http.StatusNotFound)
	if detail, _ := body(t, rec)["detail"].(string); !strings.Contains(detail, missing) {
		t.Fatalf("в detail должен быть идентификатор: %s", rec.Body.String())
	}
}

func TestRestockIncreasesStock(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/stock", `{"delta":7}`)
	expectStatus(t, rec, http.StatusOK)
	if got := body(t, rec)["stock"]; got != float64(12) {
		t.Fatalf("остаток после поступления: %v", got)
	}
}

func TestWriteOffBelowZeroIsConflict(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/stock", `{"delta":-9}`)
	expectStatus(t, rec, http.StatusConflict)
	rec = call(t, http.MethodGet, "/products/"+p.ID().String(), "")
	if got := body(t, rec)["stock"]; got != float64(5) {
		t.Fatalf("неудачное списание не должно менять остаток: %v", got)
	}
}

func TestWriteOffCannotTouchReservedGoods(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPost, "/products/"+p.ID().String()+"/reserve", `{"quantity":4}`)
	expectStatus(t, rec, http.StatusOK)
	rec = call(t, http.MethodPatch, "/products/"+p.ID().String()+"/stock", `{"delta":-3}`)
	expectStatus(t, rec, http.StatusConflict)
}

func TestZeroDeltaIsBadRequest(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/stock", `{"delta":0}`)
	expectStatus(t, rec, http.StatusBadRequest)
}

func TestMissingDeltaIsRejectedWithFieldName(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/stock", `{}`)
	expectStatus(t, rec, http.StatusBadRequest)
	errs, _ := body(t, rec)["errors"].(map[string]any)
	if errs["delta"] == nil {
		t.Fatalf("тело 400 обязано назвать поле delta: %s", rec.Body.String())
	}
}
