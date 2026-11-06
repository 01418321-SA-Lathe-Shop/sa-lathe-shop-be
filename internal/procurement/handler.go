package procurement

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if errors.Is(err, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Procurement state changed or retry key was reused")
		return
	}
	if err != nil {
		req.WriteError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, v)
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate, auth.RequireRole(auth.RoleAdmin, auth.RoleWorker))
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/suppliers", func(w http.ResponseWriter, r *http.Request) {
			var in SupplierInput
			if err := req.Decode(w, r, &in); err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.CreateSupplier(r.Context(), auth.CurrentAccount(r.Context()), in)
			respond(w, r, v, err)
		})
		r.Put("/suppliers/{supplier_id}", func(w http.ResponseWriter, r *http.Request) {
			var in SupplierUpdate
			if err := req.Decode(w, r, &in); err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.UpdateSupplier(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "supplier_id"), in)
			respond(w, r, v, err)
		})
		r.Get("/suppliers", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.Suppliers(r.Context(), auth.CurrentAccount(r.Context()), r.URL.Query().Get("search"), limit, offset)
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Get("/suppliers/recommendations", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.service.RecommendSuppliers(
				r.Context(),
				auth.CurrentAccount(r.Context()),
				r.URL.Query().Get("material_id"),
				r.URL.Query().Get("exclude_supplier_id"),
			)
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Get("/material-orders/shortages", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.service.Shortages(r.Context(), auth.CurrentAccount(r.Context()))
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Post("/material-orders", func(w http.ResponseWriter, r *http.Request) {
			var in OrderInput
			if err := req.Decode(w, r, &in); err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.Place(r.Context(), auth.CurrentAccount(r.Context()), in)
			respond(w, r, v, err)
		})
		r.Get("/material-orders", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.Orders(r.Context(), auth.CurrentAccount(r.Context()), r.URL.Query().Get("job_id"), limit, offset)
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Get("/material-orders/{order_id}", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"))
			respond(w, r, v, err)
		})
		r.Post("/material-orders/{order_id}/actions", func(w http.ResponseWriter, r *http.Request) {
			var in ActionInput
			if err := req.Decode(w, r, &in); err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.Act(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), in)
			respond(w, r, v, err)
		})
	})
}
