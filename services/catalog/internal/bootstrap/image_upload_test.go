package bootstrap_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func uploadURLPath(product uuid.UUID) string {
	return "/api/v1/products/" + product.String() + "/image-upload-url"
}

func TestImageUpload_ownerGetsPresignedURL(t *testing.T) {
	clearTables(t)
	seller := uuid.New()
	product := givenProduct(t, seller, "PUBLISHED", "1000.00")

	rec := call(t, http.MethodPost, uploadURLPath(product), sellerToken(seller), `{"contentType":"image/jpeg"}`)

	expectStatus(t, rec, http.StatusOK)
	body := decode(t, rec)
	key, _ := body["key"].(string)
	if !strings.HasPrefix(key, "products/"+product.String()+"/") {
		t.Fatalf("ключ должен говорить, чей это файл, получили %q", key)
	}
	link, _ := body["url"].(string)
	for _, part := range []string{"marketplace-images", "X-Amz-Signature", "X-Amz-Expires"} {
		if !strings.Contains(link, part) {
			t.Fatalf("ссылка должна быть временной и подписанной, нет %s: %s", part, link)
		}
	}
	if expires, _ := body["expiresAt"].(string); expires == "" {
		t.Fatalf("у ссылки нет срока жизни: %s", rec.Body.String())
	}
}

func TestImageUpload_foreignProductLooksMissing(t *testing.T) {
	clearTables(t)
	owner := uuid.New()
	product := givenProduct(t, owner, "PUBLISHED", "1000.00")

	rec := call(t, http.MethodPost, uploadURLPath(product), sellerToken(uuid.New()), `{"contentType":"image/jpeg"}`)

	expectStatus(t, rec, http.StatusNotFound)
	expectCode(t, rec, "OWN_PRODUCT_REQUIRED")
}

func TestImageUpload_unknownProductIsNotFound(t *testing.T) {
	clearTables(t)

	rec := call(t, http.MethodPost, uploadURLPath(uuid.New()), sellerToken(uuid.New()), `{"contentType":"image/jpeg"}`)

	expectStatus(t, rec, http.StatusNotFound)
	expectCode(t, rec, "PRODUCT_NOT_FOUND")
}

func TestImageUpload_anonymousIsRejected(t *testing.T) {
	clearTables(t)
	product := givenProduct(t, uuid.New(), "PUBLISHED", "1000.00")

	rec := call(t, http.MethodPost, uploadURLPath(product), "", `{"contentType":"image/jpeg"}`)

	expectStatus(t, rec, http.StatusUnauthorized)
}

func TestImageUpload_rejectsNonImageType(t *testing.T) {
	clearTables(t)
	seller := uuid.New()
	product := givenProduct(t, seller, "PUBLISHED", "1000.00")

	rec := call(t, http.MethodPost, uploadURLPath(product), sellerToken(seller), `{"contentType":"application/zip"}`)

	expectStatus(t, rec, http.StatusBadRequest)
	expectCode(t, rec, "VALIDATION_ERROR")
}
