package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) CustomerRoutes(r chi.Router, service *CustomerService) {
	r.With(NewLoginLimiter().Middleware).Post("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var in Registration
		if err := decode(w, r, &in); err != nil {
			writeAuthError(w, r, err)
			return
		}
		// Public sign-up must give an address for a new customer; internal callers (seed, tests) may omit it.
		if in.Invitation == "" && strings.TrimSpace(in.Address) == "" {
			writeAuthError(w, r, fmt.Errorf("%w: address is required", ErrInvalidInput))
			return
		}
		a, err := service.Register(r.Context(), in)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		httpserver.JSON(w, 201, a)
	})
	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate, RequireRole(RoleCustomer))
		r.Get("/customers", func(w http.ResponseWriter, r *http.Request) {
			profiles, err := service.Profiles(r.Context(), CurrentAccount(r.Context()))
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": profiles})
		})
		r.With(NewLoginLimiter().Middleware).Post("/customers/{customer_id}/invitations", func(w http.ResponseWriter, r *http.Request) {
			token, err := service.Invite(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"))
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			httpserver.JSON(w, 201, map[string]any{"invitation": token, "expires_in_seconds": 86400})
		})
		r.Get("/customers/{customer_id}/addresses", func(w http.ResponseWriter, r *http.Request) {
			items, err := service.Addresses(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"))
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": items})
		})
		r.Post("/customers/{customer_id}/addresses", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Address string `json:"address"`
			}
			if err := decode(w, r, &in); err != nil {
				writeAuthError(w, r, err)
				return
			}
			item, err := service.AddAddress(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"), in.Address)
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			httpserver.JSON(w, 201, item)
		})
		r.With(NewLoginLimiter().Middleware).Post("/customers/join", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Invitation string `json:"invitation"`
			}
			if err := decode(w, r, &in); err != nil {
				writeAuthError(w, r, err)
				return
			}
			if err := service.Join(r.Context(), CurrentAccount(r.Context()), in.Invitation); err != nil {
				writeAuthError(w, r, err)
				return
			}
			w.WriteHeader(204)
		})
	})
}
