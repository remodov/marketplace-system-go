package product

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/httpx"
)

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type Handler struct {
	service *Service
}

func Routes(r chi.Router, service *Service) {
	h := &Handler{service: service}
	r.Route("/products", func(r chi.Router) {
		r.Get("/", h.search)
		r.Post("/", h.create)
		r.Get("/{id}", h.byID)
		r.Post("/{id}/reserve", h.reserve)
	})
}

type createRequest struct {
	Title *string          `json:"title"`
	Price *decimal.Decimal `json:"price"`
	Stock *int             `json:"stock"`
}

type reserveRequest struct {
	Quantity *int `json:"quantity"`
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	var (
		found []*Product
		err   error
	)
	if raw := r.URL.Query().Get("maxPrice"); raw != "" {
		maxPrice, parseErr := decimal.NewFromString(raw)
		if parseErr != nil || maxPrice.Sign() <= 0 {
			httpx.WriteFieldErrors(w, r, map[string]string{"maxPrice": "должна быть положительным числом"})
			return
		}
		found, err = h.service.CheaperThan(r.Context(), maxPrice)
	} else {
		found, err = h.service.Search(r.Context(), r.URL.Query().Get("query"))
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards(found))
}

func (h *Handler) byID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	card, err := h.service.Card(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, card)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decode(w, r, &req) {
		return
	}
	errs := map[string]string{}
	if req.Title == nil || *req.Title == "" {
		errs["title"] = "название обязательно"
	}
	if req.Price == nil {
		errs["price"] = "цена обязательна"
	} else if req.Price.Sign() <= 0 {
		errs["price"] = "цена должна быть больше нуля"
	}
	if req.Stock == nil {
		errs["stock"] = "остаток обязателен"
	} else if *req.Stock < 0 {
		errs["stock"] = "остаток не может быть отрицательным"
	}
	if len(errs) > 0 {
		httpx.WriteFieldErrors(w, r, errs)
		return
	}
	p, err := h.service.Create(r.Context(), *req.Title, *req.Price, *req.Stock)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, CardOf(p))
}

func (h *Handler) reserve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req reserveRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Quantity == nil || *req.Quantity <= 0 {
		httpx.WriteFieldErrors(w, r, map[string]string{"quantity": "количество должно быть больше нуля"})
		return
	}
	p, err := h.service.Reserve(r.Context(), id, *req.Quantity)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, CardOf(p))
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var outOfStock *OutOfStockError
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, err.Error())
	case errors.As(err, &outOfStock):
		httpx.WriteProblem(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, ErrConflict):
		httpx.WriteProblem(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalid):
		httpx.WriteProblem(w, r, http.StatusBadRequest, err.Error())
	default:
		slog.Error("необработанная ошибка", "path", r.URL.Path, "err", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "Внутренняя ошибка сервиса")
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Идентификатор товара должен быть UUID")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "Тело запроса не разобрать: "+err.Error())
		return false
	}
	return true
}

func cards(products []*Product) []Card {
	out := make([]Card, 0, len(products))
	for _, p := range products {
		out = append(out, CardOf(p))
	}
	return out
}
