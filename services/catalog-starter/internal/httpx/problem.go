package httpx

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   map[string]string `json:"errors,omitempty"`
}

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, detail string) {
	writeProblem(w, r, Problem{Status: status, Detail: detail})
}

func WriteFieldErrors(w http.ResponseWriter, r *http.Request, errs map[string]string) {
	writeProblem(w, r, Problem{Status: http.StatusBadRequest, Detail: "Запрос не прошёл проверку", Errors: errs})
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, r *http.Request, p Problem) {
	p.Type = "about:blank"
	p.Title = http.StatusText(p.Status)
	p.Instance = r.URL.Path
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
