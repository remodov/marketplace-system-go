package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewRouter(auth Authenticator, orders *OrderHandler, db Pinger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeProblem(w, r, http.StatusServiceUnavailable, "NOT_READY", "База недоступна")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Group(func(r chi.Router) {
		r.Use(Authenticate(auth))
		orders.Routes(r)
	})
	return r
}
