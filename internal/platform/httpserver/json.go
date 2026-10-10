package httpserver

import (
	"encoding/json"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, map[string]any{"error": Error{Code: code, Message: message, RequestID: middleware.GetReqID(r.Context())}})
}
