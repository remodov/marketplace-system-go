package payment

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type AuthorizeRequest struct {
	OrderID  *uuid.UUID       `json:"orderId"`
	Amount   *decimal.Decimal `json:"amount"`
	Currency *string          `json:"currency"`
}

type View struct {
	ID        uuid.UUID       `json:"id"`
	OrderID   uuid.UUID       `json:"orderId"`
	Amount    decimal.Decimal `json:"amount"`
	Currency  string          `json:"currency"`
	Status    string          `json:"status"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}

type Pinger interface {
	PingContext(ctx context.Context) error
}

func NewRouter(service *Service, db Pinger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "NOT_READY", "База недоступна")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := &handler{service: service}
	r.Route("/api/v1/payments", func(r chi.Router) {
		r.Post("/", h.authorize)
		r.Get("/{id}", h.byID)
		r.Post("/{id}/capture", h.capture)
		r.Post("/{id}/refund", h.refund)
	})
	return r
}

type handler struct {
	service *Service
}

func (h *handler) authorize(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "MALFORMED_REQUEST", "Невозможно разобрать тело запроса")
		return
	}
	if req.OrderID == nil || req.Amount == nil || req.Amount.Sign() <= 0 || req.Currency == nil || len(*req.Currency) != 3 {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "Нужны orderId, сумма больше нуля и валюта из трёх букв")
		return
	}
	p, err := h.service.Authorize(r.Context(), *req.OrderID, *req.Amount, *req.Currency)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, view(p))
}

func (h *handler) byID(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(id uuid.UUID) (Payment, error) { return h.service.ByID(r.Context(), id) })
}

func (h *handler) capture(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(id uuid.UUID) (Payment, error) { return h.service.Capture(r.Context(), id) })
}

func (h *handler) refund(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, func(id uuid.UUID) (Payment, error) { return h.service.Refund(r.Context(), id) })
}

func (h *handler) respond(w http.ResponseWriter, r *http.Request, op func(id uuid.UUID) (Payment, error)) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "Идентификатор платежа должен быть UUID")
		return
	}
	p, err := op(id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view(p))
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var transition *InvalidTransitionError
	switch {
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "PAYMENT_NOT_FOUND", "Платёж не найден")
	case errors.As(err, &transition):
		writeProblem(w, http.StatusConflict, "INVALID_PAYMENT_TRANSITION", transition.Error())
	default:
		slog.ErrorContext(r.Context(), "необработанная ошибка", "err", err, "path", r.URL.Path)
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Внутренняя ошибка сервиса")
	}
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{Type: "urn:problem:payment:" + code, Title: http.StatusText(status), Status: status, Detail: detail, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func view(p Payment) View {
	return View{ID: p.ID, OrderID: p.OrderID, Amount: p.Amount, Currency: p.Currency, Status: string(p.Status), UpdatedAt: p.UpdatedAt.UTC()}
}
