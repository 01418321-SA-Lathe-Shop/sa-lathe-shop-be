package delivery

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct {
	service  *Service
	evidence *EvidenceService
}

func NewHandler(s *Service, e *EvidenceService) *Handler { return &Handler{s, e} }
func respond(w http.ResponseWriter, r *http.Request, v any, e error) {
	if errors.Is(e, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Fulfillment state or command has changed")
		return
	}
	if e != nil {
		req.WriteError(w, r, e)
		return
	}
	httpserver.JSON(w, 200, v)
}
func endpoint[T any](fn func(*http.Request, T) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in T
		if e := req.Decode(w, r, &in); e != nil {
			respond(w, r, nil, e)
			return
		}
		v, e := fn(r, in)
		respond(w, r, v, e)
	}
}
func (h *Handler) Routes(router chi.Router, authenticate func(http.Handler) http.Handler) {
	router.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/fulfillment/jobs", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, e := req.Pagination(r)
			if e != nil {
				respond(w, r, nil, e)
				return
			}
			v, e := h.service.List(r.Context(), auth.CurrentAccount(r.Context()), limit, offset)
			respond(w, r, map[string]any{"items": v}, e)
		})
		r.Get("/fulfillment/jobs/{job_id}", func(w http.ResponseWriter, r *http.Request) {
			v, e := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"))
			respond(w, r, v, e)
		})
		r.With(auth.RequireRole(auth.RoleAdmin, auth.RoleWorker)).Put("/fulfillment/jobs/{job_id}/credit-terms", endpoint(func(r *http.Request, in TermsInput) (any, error) {
			return h.service.Terms(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.Post("/fulfillment/jobs/{job_id}/invoice", endpoint(func(r *http.Request, in Command) (any, error) {
			return h.service.Issue(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.Post("/fulfillment/jobs/{job_id}/deliveries", endpoint(func(r *http.Request, in ShipInput) (any, error) {
			return h.service.Ship(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.With(auth.RequireRole(auth.RoleCustomer)).Post("/fulfillment/jobs/{job_id}/inspection", endpoint(func(r *http.Request, in InspectInput) (any, error) {
			return h.service.Inspect(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.With(auth.RequireRole(auth.RoleCustomer)).Post("/fulfillment/jobs/{job_id}/payments/bank", endpoint(func(r *http.Request, in PaymentInput) (any, error) {
			return h.service.Pay(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in, false)
		}))
		r.With(auth.RequireRole(auth.RoleAdmin, auth.RoleWorker)).Post("/fulfillment/jobs/{job_id}/payments/cash", endpoint(func(r *http.Request, in PaymentInput) (any, error) {
			return h.service.Pay(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in, true)
		}))
		r.With(auth.RequireRole(auth.RoleAdmin, auth.RoleWorker)).Post("/fulfillment/jobs/{job_id}/payments/review", endpoint(func(r *http.Request, in ReviewInput) (any, error) {
			return h.service.Review(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.Post("/fulfillment/jobs/{job_id}/evidence/uploads", endpoint(func(r *http.Request, in file.Input) (any, error) {
			return h.evidence.Start(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		r.Post("/customer-evidence/{evidence_id}/complete", func(w http.ResponseWriter, r *http.Request) {
			v, e := h.evidence.Complete(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "evidence_id"))
			respond(w, r, v, e)
		})
		r.Get("/customer-evidence/{evidence_id}/download", func(w http.ResponseWriter, r *http.Request) {
			v, e := h.evidence.Download(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "evidence_id"))
			respond(w, r, v, e)
		})
	})
}
