package auth

import (
	"net/http"

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
