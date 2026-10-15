package quotation

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "An unexpected error occurred"
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, req.ErrInvalidInput):
		status, code, message = 400, "invalid_input", "Invalid quotation input or price below minimum"
	case errors.Is(err, ErrForbidden):
		status, code, message = 403, "forbidden", "Permission denied"
	case errors.Is(err, ErrNotFound):
		status, code, message = 404, "not_found", "Quotation or request not found"
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "conflict", "Quotation or request changed; reload before retrying"
	}
	httpserver.WriteError(w, r, status, code, message)
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/quotations/estimate", func(w http.ResponseWriter, r *http.Request) {
			var in DraftInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Estimate(r.Context(), auth.CurrentAccount(r.Context()), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Post("/requests/{request_id}/quotations", func(w http.ResponseWriter, r *http.Request) {
			var in DraftInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Create(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "request_id"), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 201, result)
		})
		r.Patch("/quotations/{quotation_id}", func(w http.ResponseWriter, r *http.Request) {
			var in EditInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Edit(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Post("/quotations/{quotation_id}/send", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Expected int64 `json:"expected_lock_version"`
			}
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Send(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"), in.Expected)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Post("/requests/{request_id}/reject", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Reason string `json:"reason"`
			}
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			if err := h.service.Reject(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "request_id"), in.Reason); err != nil {
				writeError(w, r, err)
				return
			}
			w.WriteHeader(204)
		})
		r.Get("/quotations/{quotation_id}", func(w http.ResponseWriter, r *http.Request) {
			result, err := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"))
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Get("/requests/{request_id}/quotations", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				writeError(w, r, err)
				return
			}
			items, err := h.service.List(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "request_id"), limit, offset)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": items, "limit": limit, "offset": offset})
		})
	})
}
