package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/bff/internal/ratelimit"
	"github.com/remodov/marketplace-system-go/services/bff/internal/screen"
)

func NewRouter(limiter *ratelimit.Limiter, screens *screen.Assembler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Group(func(r chi.Router) {
		r.Use(ratelimit.Middleware(limiter))
		r.Get("/api/v1/screens/order/{orderId}", orderScreen(screens))
	})
	return r
}

func orderScreen(screens *screen.Assembler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "orderId должен быть UUID")
			return
		}
		result, err := screens.Assemble(r.Context(), orderID, r.Header.Get("Authorization"))
		if err != nil {
			writeDownstream(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeDownstream(w http.ResponseWriter, err error) {
	var downstream *screen.DownstreamError
	if errors.As(err, &downstream) && downstream.Service == "order" {
		switch downstream.Status {
		case http.StatusNotFound:
			writeProblem(w, http.StatusNotFound, "ORDER_NOT_FOUND", "Заказ не найден")
			return
		case http.StatusUnauthorized, http.StatusForbidden:
			writeProblem(w, downstream.Status, "ORDER_ACCESS_DENIED", "Заказ недоступен этому пользователю")
			return
		}
	}
	slog.Warn("экран заказа не собран", "err", err)
	writeProblem(w, http.StatusBadGateway, "DOWNSTREAM_UNAVAILABLE", "Сервис-источник не ответил, экран собрать не удалось")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "title": http.StatusText(status), "code": code, "detail": detail})
}
