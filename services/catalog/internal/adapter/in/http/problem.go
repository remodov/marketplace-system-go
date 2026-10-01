package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
)

type Problem struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Code     string            `json:"code"`
	Errors   map[string]string `json:"errors,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, detail string) {
	writeProblemWithErrors(w, r, status, code, detail, nil)
}

func writeProblemWithErrors(w http.ResponseWriter, r *http.Request, status int, code, detail string, errs map[string]string) {
	p := Problem{
		Type:     "urn:problem:catalog:" + code,
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
		Errors:   errs,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var e *apperr.Error
	if errors.As(err, &e) {
		writeProblem(w, r, statusOf(e.Kind), e.Code, e.Message)
		return
	}
	slog.ErrorContext(r.Context(), "необработанная ошибка", "err", err, "path", r.URL.Path)
	writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Внутренняя ошибка сервиса")
}

func statusOf(kind apperr.Kind) int {
	switch kind {
	case apperr.KindInvalid:
		return http.StatusBadRequest
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
