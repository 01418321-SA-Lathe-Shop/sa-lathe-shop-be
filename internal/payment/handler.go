package payment

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{service: s} }
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Evidence or fulfillment state has changed")
		return
	}
	req.WriteError(w, r, err)
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/quotations/{quotation_id}/evidence", func(w http.ResponseWriter, r *http.Request) {
			var in Submission
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Submit(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Post("/evidence/{evidence_id}/review", func(w http.ResponseWriter, r *http.Request) {
			var in Review
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.Review(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "evidence_id"), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
		r.Get("/quotations/{quotation_id}/fulfillment", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			result, err := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"))
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, result)
		})
	})
}
