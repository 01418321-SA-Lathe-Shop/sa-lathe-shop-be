package receiving

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
func respond(w http.ResponseWriter, r *http.Request, v any, err error) {
	if errors.Is(err, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Order state or command has changed")
		return
	}
	if err != nil {
		req.WriteError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, v)
}
func jsonEndpoint[T any](fn func(*http.Request, T) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in T
		if err := req.Decode(w, r, &in); err != nil {
			respond(w, r, nil, err)
			return
		}
		v, err := fn(r, in)
		respond(w, r, v, err)
	}
}
func (h *Handler) Routes(router chi.Router, authenticate func(http.Handler) http.Handler) {
	router.Group(func(router chi.Router) {
		router.Use(authenticate)
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if err := permitted(auth.CurrentAccount(r.Context()), false); err != nil {
					respond(w, r, nil, err)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		router.Get("/receiving/orders", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				respond(w, r, nil, err)
				return
			}
			v, err := h.service.List(r.Context(), auth.CurrentAccount(r.Context()), limit, offset)
			respond(w, r, map[string]any{"items": v}, err)
		})
		router.Get("/receiving/orders/{order_id}", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"))
			respond(w, r, v, err)
		})
		router.Post("/receiving/orders/{order_id}/receipts", jsonEndpoint(func(r *http.Request, in ReceiptInput) (any, error) {
			return h.service.Receive(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), in)
		}))
		router.Post("/receiving/orders/{order_id}/claims/{claim_id}/submit", jsonEndpoint(func(r *http.Request, in ClaimInput) (any, error) {
			return h.service.Claim(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), chi.URLParam(r, "claim_id"), in)
		}))
		router.Group(func(router chi.Router) {
			router.Use(auth.RequireRole(auth.RoleAdmin))
			router.Post("/receiving/orders/{order_id}/credit-terms", jsonEndpoint(func(r *http.Request, in CreditInput) (any, error) {
				return h.service.Credit(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), in)
			}))
			router.Post("/receiving/orders/{order_id}/settlement", jsonEndpoint(func(r *http.Request, in SettlementInput) (any, error) {
				return h.service.Settle(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), in)
			}))
			router.Post("/material-orders/{order_id}/evidence/uploads", jsonEndpoint(func(r *http.Request, in file.Input) (any, error) {
				return h.evidence.Start(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "order_id"), in)
			}))
			router.Post("/order-evidence/{evidence_id}/complete", func(w http.ResponseWriter, r *http.Request) {
				v, err := h.evidence.Complete(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "evidence_id"))
				respond(w, r, v, err)
			})
			router.Get("/order-evidence/{evidence_id}/download", func(w http.ResponseWriter, r *http.Request) {
				v, err := h.evidence.Download(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "evidence_id"))
				respond(w, r, v, err)
			})
		})
	})
}
