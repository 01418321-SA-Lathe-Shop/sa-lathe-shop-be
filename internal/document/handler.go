package document

import (
	"errors"
	"net/http"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "An unexpected error occurred"
	switch {
	case errors.Is(err, ErrInvalidInput), errors.Is(err, req.ErrInvalidInput):
		status, code, message = 400, "invalid_input", "Invalid document input"
	case errors.Is(err, ErrForbidden):
		status, code, message = 403, "forbidden", "Permission denied"
	case errors.Is(err, ErrNotFound):
		status, code, message = 404, "not_found", "Document not found"
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "conflict", "Document changed or can no longer be edited; reload before retrying"
	}
	if status == http.StatusInternalServerError {
		httpserver.LogInternalError(r, err)
	}
	httpserver.WriteError(w, r, status, code, message)
}

// respond writes result as JSON or the mapped error.
func respond[T any](w http.ResponseWriter, r *http.Request, result T, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, result)
}

func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Get("/shop-profile", func(w http.ResponseWriter, r *http.Request) {
			result, err := h.service.ShopProfile(r.Context(), auth.CurrentAccount(r.Context()))
			respond(w, r, result, err)
		})
		r.Put("/shop-profile", func(w http.ResponseWriter, r *http.Request) {
			var in ShopInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.SaveShopProfile(r.Context(), auth.CurrentAccount(r.Context()), in)
			respond(w, r, result, err)
		})
		r.Get("/customers/{customer_id}/billing", func(w http.ResponseWriter, r *http.Request) {
			result, err := h.service.Billing(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"))
			respond(w, r, result, err)
		})
		r.Put("/customers/{customer_id}/billing", func(w http.ResponseWriter, r *http.Request) {
			var in BillingInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.SaveBilling(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"), in)
			respond(w, r, result, err)
		})
		r.Get("/quotations/{quotation_id}/customer-po", func(w http.ResponseWriter, r *http.Request) {
			result, err := h.service.CustomerPO(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"))
			respond(w, r, result, err)
		})
		r.Put("/quotations/{quotation_id}/customer-po", func(w http.ResponseWriter, r *http.Request) {
			var in POInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			result, err := h.service.SaveCustomerPO(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "quotation_id"), in)
			respond(w, r, result, err)
		})
	})
}
