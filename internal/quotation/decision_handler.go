package quotation

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type DecisionHandler struct{ service *DecisionService }

func NewDecisionHandler(service *DecisionService) *DecisionHandler {
	return &DecisionHandler{service: service}
}
func (h *DecisionHandler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/quotations/{quotation_id}/decision", func(w http.ResponseWriter, r *http.Request) {
			var in DecisionInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			value, err := h.service.Decide(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"), in)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, value)
		})
		r.Get("/quotations/{quotation_id}/decision", func(w http.ResponseWriter, r *http.Request) {
			value, err := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"))
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, value)
		})
		r.Get("/requests/{request_id}/quotation-decisions", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				writeError(w, r, err)
				return
			}
			values, err := h.service.List(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "request_id"), limit, offset)
			if err != nil {
				writeError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": values, "limit": limit, "offset": offset})
		})
	})
}
