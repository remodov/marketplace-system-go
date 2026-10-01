package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/query"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/usecase"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type ProductHandler struct {
	create  *usecase.CreateProductHandler
	price   *usecase.ChangeProductPriceHandler
	status  *usecase.ChangeStatusHandler
	queries *query.Handler
}

func NewProductHandler(create *usecase.CreateProductHandler, price *usecase.ChangeProductPriceHandler, status *usecase.ChangeStatusHandler, queries *query.Handler) *ProductHandler {
	return &ProductHandler{create: create, price: price, status: status, queries: queries}
}

func (h *ProductHandler) Routes(r chi.Router) {
	r.Route("/api/v1/products", func(r chi.Router) {
		r.Get("/{productId}", h.getProduct)
		r.Group(func(r chi.Router) {
			r.Use(RequireRoles(security.RoleSeller, security.RoleAdmin))
			r.Post("/", h.createProduct)
			r.Get("/my", h.listMyProducts)
			r.Post("/{productId}/publish", h.publishProduct)
			r.Post("/{productId}/hide", h.hideProduct)
			r.Patch("/{productId}/price", h.changeProductPrice)
		})
	})
}

func (h *ProductHandler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Невозможно разобрать тело запроса")
		return
	}
	errs := map[string]string{}
	if req.Title == nil || strings.TrimSpace(*req.Title) == "" {
		errs["title"] = "обязательное поле"
	}
	if req.Price == nil || req.Price.Sign() <= 0 {
		errs["price"] = "должна быть больше нуля"
	}
	if req.Currency == nil || *req.Currency == "" {
		errs["currency"] = "обязательное поле"
	}
	if len(errs) > 0 {
		writeProblemWithErrors(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ошибка валидации входных данных", errs)
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	product, err := h.create.Handle(r.Context(), usecase.CreateProduct{
		Seller: principal, Title: *req.Title, Description: req.Description, Price: *req.Price, Currency: *req.Currency,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/products/"+product.ID().String())
	writeJSON(w, http.StatusCreated, toDTO(product))
}

func (h *ProductHandler) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}
	var requester *security.Principal
	if p, ok := security.PrincipalFrom(r.Context()); ok {
		requester = &p
	}
	product, err := h.queries.GetProduct(r.Context(), query.GetProduct{ProductID: id, Requester: requester})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(product))
}

func (h *ProductHandler) publishProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	product, err := h.status.Publish(r.Context(), usecase.PublishProduct{ProductID: id, Requester: principal})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(product))
}

func (h *ProductHandler) hideProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	product, err := h.status.Hide(r.Context(), usecase.HideProduct{ProductID: id, Requester: principal})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(product))
}

func (h *ProductHandler) changeProductPrice(w http.ResponseWriter, r *http.Request) {
	id, ok := productID(w, r)
	if !ok {
		return
	}
	var req ChangePriceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Невозможно разобрать тело запроса")
		return
	}
	if req.Price == nil || req.Price.Sign() <= 0 {
		writeProblemWithErrors(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ошибка валидации входных данных", map[string]string{"price": "должна быть больше нуля"})
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	product, err := h.price.Handle(r.Context(), usecase.ChangeProductPrice{ProductID: id, Requester: principal, NewPrice: *req.Price})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(product))
}

func (h *ProductHandler) listMyProducts(w http.ResponseWriter, r *http.Request) {
	principal, _ := security.PrincipalFrom(r.Context())
	q := r.URL.Query()
	filter := out.ListFilter{Page: atoiOr(q.Get("page"), 1), Size: atoiOr(q.Get("size"), 20), Sort: out.SortField(q.Get("sort"))}
	if raw := q.Get("status"); raw != "" {
		status, ok := aggregate.ParseStatus(raw)
		if !ok {
			writeProblemWithErrors(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ошибка валидации входных данных", map[string]string{"status": "DRAFT, PUBLISHED или HIDDEN"})
			return
		}
		filter.Status = &status
	}
	page, err := h.queries.ListMyProducts(r.Context(), query.ListMyProducts{Seller: principal.Sub, Filter: filter})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toPageDTO(page))
}

func productID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "productId"))
	if err != nil {
		writeProblemWithErrors(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ошибка валидации входных данных", map[string]string{"productId": "должен быть UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func atoiOr(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
