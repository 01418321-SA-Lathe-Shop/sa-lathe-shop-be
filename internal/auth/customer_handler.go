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
		// Public sign-up must give an address (flat or structured) for a new customer; internal callers (seed, tests) may omit it.
		if in.Invitation == "" && strings.TrimSpace(in.Address) == "" && strings.TrimSpace(in.AddressLine1) == "" {
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
				Address      string `json:"address"`
				AddressLine1 string `json:"address_line1"`
				AddressLine2 string `json:"address_line2"`
				District     string `json:"district"`
				Province     string `json:"province"`
				PostalCode   string `json:"postal_code"`
				IsDefault    bool   `json:"is_default"`
			}
			if err := decode(w, r, &in); err != nil {
				writeAuthError(w, r, err)
				return
			}
			item, err := service.AddStructuredAddress(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "customer_id"), CustomerAddress{
				Address:      in.Address,
				AddressLine1: in.AddressLine1,
				AddressLine2: in.AddressLine2,
				District:     in.District,
				Province:     in.Province,
				PostalCode:   in.PostalCode,
				IsDefault:    in.IsDefault,
			})
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
