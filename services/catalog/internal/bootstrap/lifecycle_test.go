package bootstrap_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreatePublishHide_asSeller(t *testing.T) {
	clearTables(t)
	seller := uuid.New()

	created := call(t, http.MethodPost, "/api/v1/products", sellerToken(seller), `{"title": "Кофемолка", "description": "ручная", "price": 2490.5, "currency": "RUB"}`)
	expectStatus(t, created, http.StatusCreated)
	body := decode(t, created)
	if body["status"] != "DRAFT" {
		t.Fatalf("новый товар не черновик: %v", body)
	}
	id := body["id"].(string)
	if created.Header().Get("Location") != "/api/v1/products/"+id {
		t.Fatalf("Location: %s", created.Header().Get("Location"))
	}

	hidden := call(t, http.MethodGet, "/api/v1/products/"+id, "", "")
	expectStatus(t, hidden, http.StatusNotFound)
	expectCode(t, hidden, "PRODUCT_NOT_FOUND")

	own := call(t, http.MethodGet, "/api/v1/products/"+id, sellerToken(seller), "")
	expectStatus(t, own, http.StatusOK)

	published := call(t, http.MethodPost, "/api/v1/products/"+id+"/publish", sellerToken(seller), "")
	expectStatus(t, published, http.StatusOK)
	if decode(t, published)["status"] != "PUBLISHED" {
		t.Fatal("после публикации статус не PUBLISHED")
	}

	public := call(t, http.MethodGet, "/api/v1/products/"+id, "", "")
	expectStatus(t, public, http.StatusOK)

	again := call(t, http.MethodPost, "/api/v1/products/"+id+"/publish", sellerToken(seller), "")
	expectStatus(t, again, http.StatusConflict)
	expectCode(t, again, "INVALID_STATE_TRANSITION")

	hide := call(t, http.MethodPost, "/api/v1/products/"+id+"/hide", sellerToken(seller), "")
	expectStatus(t, hide, http.StatusOK)

	mine := call(t, http.MethodGet, "/api/v1/products/my?status=HIDDEN", sellerToken(seller), "")
	expectStatus(t, mine, http.StatusOK)
	if total := decode(t, mine)["total"]; total != 1.0 {
		t.Fatalf("в списке продавца %v скрытых", total)
	}
	if len(auditActions(t)) != 0 {
		t.Fatalf("действия продавца попали в журнал администратора: %v", auditActions(t))
	}
}

func TestCreateProduct_validation(t *testing.T) {
	clearTables(t)
	seller := uuid.New()

	rec := call(t, http.MethodPost, "/api/v1/products", sellerToken(seller), `{"title": "", "price": -1, "currency": "USD"}`)
	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "VALIDATION_ERROR")

	rec = call(t, http.MethodPost, "/api/v1/products", sellerToken(seller), `{"title": "Кружка", "price": 100, "currency": "USD"}`)
	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "INVALID_CURRENCY")

	rec = call(t, http.MethodPost, "/api/v1/products", "customer."+uuid.NewString(), `{"title": "Кружка", "price": 100, "currency": "RUB"}`)
	expectStatus(t, rec, http.StatusForbidden)
	expectCode(t, rec, "ACCESS_DENIED")
}
