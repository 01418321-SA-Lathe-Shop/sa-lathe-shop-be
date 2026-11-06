package request

import (
	"errors"
	"net/http"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func principal(r *http.Request) Principal {
	a := auth.CurrentAccount(r.Context())
	return Principal{AccountID: a.ID, Role: a.Role, Active: a.Active}
}
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	if httpserver.DecodeJSON(w, r, v, 64*1024) != nil {
		return ErrInvalidInput
	}
	return nil
}
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "An unexpected error occurred"
	switch {
	case errors.Is(err, ErrInvalidInput):
		status, code, message = 400, "invalid_input", "Invalid request input"
	case errors.Is(err, ErrForbidden):
		status, code, message = 403, "forbidden", "Permission denied"
	case errors.Is(err, ErrNotFound):
		status, code, message = 404, "not_found", "Request or file not found"
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "conflict", "Request state changed"
	}
	if status == http.StatusInternalServerError {
		httpserver.LogInternalError(r, err)
	}
	httpserver.WriteError(w, r, status, code, message)
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/requests", func(w http.ResponseWriter, r *http.Request) {
			var in Input
			if err := Decode(w, r, &in); err != nil {
				WriteError(w, r, err)
				return
			}
			result, err := h.service.Submit(r.Context(), principal(r), in)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 201, result)
		})
		r.Get("/requests/{request_id}", func(w http.ResponseWriter, r *http.Request) {
			result, err := h.service.Get(r.Context(), principal(r), chi.URLParam(r, "request_id"))
			if err != nil {
				WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Delete("/requests/{request_id}", func(w http.ResponseWriter, r *http.Request) {
			if err := h.service.RollbackPending(r.Context(), principal(r), chi.URLParam(r, "request_id")); err != nil {
				WriteError(w, r, err)
				return
			}
			w.WriteHeader(204)
		})
		r.Post("/requests/{request_id}/request-info", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Note string `json:"note"`
			}
			if err := Decode(w, r, &in); err != nil {
				WriteError(w, r, err)
				return
			}
			result, err := h.service.RequestInfo(r.Context(), principal(r), chi.URLParam(r, "request_id"), in.Note)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Patch("/requests/{request_id}", func(w http.ResponseWriter, r *http.Request) {
			var in Correction
			if err := Decode(w, r, &in); err != nil {
				WriteError(w, r, err)
				return
			}
			result, err := h.service.Resubmit(r.Context(), principal(r), chi.URLParam(r, "request_id"), in)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Get("/requests", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := Pagination(r)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			items, err := h.service.List(r.Context(), principal(r), limit, offset)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": items, "limit": limit, "offset": offset})
		})
	})
}
func Pagination(r *http.Request) (int, int, error) {
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		return 0, 0, ErrInvalidInput
	}
	return limit, offset, nil
}
