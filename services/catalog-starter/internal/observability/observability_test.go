package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/observability"
)

func newRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(observability.Metrics("catalog-starter"))
	observability.Mount(r, func(context.Context) error { return nil })
	r.Get("/products", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	return r
}

func get(router http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestReadinessProbeAnswers(t *testing.T) {
	if rec := get(newRouter(), "/health/ready"); rec.Code != http.StatusNoContent {
		t.Fatalf("проба готовности: ожидали 204, получили %d", rec.Code)
	}
}

func TestLivenessProbeAnswers(t *testing.T) {
	if rec := get(newRouter(), "/health/live"); rec.Code != http.StatusNoContent {
		t.Fatalf("проба живости: ожидали 204, получили %d", rec.Code)
	}
}

func TestPrometheusMetricsCarryServiceLabel(t *testing.T) {
	router := newRouter()
	get(router, "/products")

	rec := get(router, "/metrics")

	if rec.Code != http.StatusOK {
		t.Fatalf("метрики: ожидали 200, получили %d", rec.Code)
	}
	body := rec.Body.String()
	for _, part := range []string{"http_server_request_duration_seconds", `service="catalog-starter"`, `route="/products"`} {
		if !strings.Contains(body, part) {
			t.Fatalf("в метриках нет %s", part)
		}
	}
}

func TestSamplerKeepsEveryTraceWhenRatioIsOne(t *testing.T) {
	if got := observability.Sampler(1.0).Description(); !strings.Contains(got, "TraceIDRatioBased{1}") {
		t.Fatalf("сэмплер должен брать все трассы при доле 1.0, получили %s", got)
	}
}
