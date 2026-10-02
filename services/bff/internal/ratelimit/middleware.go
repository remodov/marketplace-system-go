package ratelimit

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

const clientHeader = "X-Client-Id"

func Middleware(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := strings.TrimSpace(r.Header.Get(clientHeader))
			if client == "" {
				client = "anonymous"
			}
			decision, err := limiter.Check(r.Context(), client)
			if err != nil {
				slog.Warn("лимит частоты не проверен, запрос пропущен", "client", client, "err", err)
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter.Seconds())))
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"status":429,"title":"Too Many Requests","code":"RATE_LIMITED","detail":"Слишком много запросов, попробуйте позже"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
