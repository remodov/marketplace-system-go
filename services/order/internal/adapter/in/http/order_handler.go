package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/query"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/usecase"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/security"
)

type OrderHandler struct {
	create  *usecase.CreateOrderHandler
	queries *query.Handler
}

func NewOrderHandler(create *usecase.CreateOrderHandler, queries *query.Handler) *OrderHandler {
	return &OrderHandler{create: create, queries: queries}
}

func (h *OrderHandler) Routes(r chi.Router) {
	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Use(RequireRoles(security.RoleCustomer, security.RoleAdmin))
		r.Post("/", h.createOrder)
		r.Get("/{orderId}", h.getOrder)
	})
}

func (h *OrderHandler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Невозможно разобрать тело запроса")
		return
	}
	lines, errs := validateLines(req.Items)
	if req.ShippingAddress == nil || strings.TrimSpace(req.ShippingAddress.City) == "" || strings.TrimSpace(req.ShippingAddress.Street) == "" {
		errs["shippingAddress"] = "нужны город и улица"
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		errs["Idempotency-Key"] = "обязательный заголовок до 128 символов"
	}
	if len(errs) > 0 {
		writeProblemWithErrors(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Ошибка валидации входных данных", errs)
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	result, err := h.create.Handle(r.Context(), usecase.CreateOrder{
		Customer: principal, Lines: lines, ShippingAddress: toAddress(*req.ShippingAddress),
		IdempotencyKey: idempotencyKey, RequestHash: requestHash(req),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !result.Created {
		writeJSON(w, http.StatusOK, toDTO(result.Order))
		return
	}
	w.Header().Set("Location", "/api/v1/orders/"+result.Order.ID().String())
	writeJSON(w, http.StatusCreated, toDTO(result.Order))
}

func requestHash(req CreateOrderRequest) string {
	canonical, _ := json.Marshal(req)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func (h *OrderHandler) getOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Идентификатор заказа должен быть UUID")
		return
	}
	principal, _ := security.PrincipalFrom(r.Context())
	order, err := h.queries.GetOrder(r.Context(), query.GetOrder{OrderID: id, Requester: principal})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDTO(order))
}

func validateLines(items []OrderItemRequest) ([]usecase.OrderLine, map[string]string) {
	errs := map[string]string{}
	if len(items) == 0 {
		errs["items"] = "нужна хотя бы одна позиция"
		return nil, errs
	}
	lines := make([]usecase.OrderLine, 0, len(items))
	for i, item := range items {
		field := fmt.Sprintf("items[%d]", i)
		if item.ProductID == nil {
			errs[field+".productId"] = "обязательное поле"
		}
		if item.SellerID == nil {
			errs[field+".sellerId"] = "обязательное поле"
		}
		if item.Quantity == nil || *item.Quantity < 1 || *item.Quantity > aggregate.MaxQuantity {
			errs[field+".quantity"] = fmt.Sprintf("от 1 до %d", aggregate.MaxQuantity)
		}
		if item.ProductID != nil && item.SellerID != nil && item.Quantity != nil {
			lines = append(lines, usecase.OrderLine{ProductID: *item.ProductID, SellerID: *item.SellerID, Quantity: *item.Quantity})
		}
	}
	return lines, errs
}
