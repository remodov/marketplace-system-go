package product_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestDiscountIsApplied(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/discount", `{"percent":20}`)
	expectStatus(t, rec, http.StatusOK)
	if got := body(t, rec)["price"]; got != float64(1592) {
		t.Fatalf("цена после скидки 20%%: %v", got)
	}
}

func TestTooDeepDiscountIsRejected(t *testing.T) {
	p := mustCreate(t, unique("Беспроводная мышь"), "1990.00", 5)
	rec := call(t, http.MethodPatch, "/products/"+p.ID().String()+"/discount", `{"percent":80}`)
	expectStatus(t, rec, http.StatusBadRequest)
	if detail, _ := body(t, rec)["detail"].(string); !strings.Contains(detail, "50") {
		t.Fatalf("отказ должен назвать предел скидки: %s", rec.Body.String())
	}
	rec = call(t, http.MethodGet, "/products/"+p.ID().String(), "")
	if got := body(t, rec)["price"]; got != float64(1990) {
		t.Fatalf("неудачная скидка изменила цену: %v", got)
	}
}
