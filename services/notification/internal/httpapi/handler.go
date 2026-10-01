package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/notification/internal/inbox"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewRouter(processor *inbox.Processor, db Pinger, adminToken string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/api/v1/notifications", func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != adminToken {
			http.Error(w, `{"code":"ACCESS_DENIED"}`, http.StatusForbidden)
			return
		}
		userID, err := uuid.Parse(r.URL.Query().Get("userId"))
		if err != nil {
			http.Error(w, `{"code":"VALIDATION_ERROR","detail":"userId должен быть UUID"}`, http.StatusBadRequest)
			return
		}
		items, err := processor.ListByUser(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"code":"INTERNAL_SERVER_ERROR"}`, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []inbox.Notification{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	return r
}
