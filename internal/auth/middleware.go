package auth

import (
	"context"
	"net/http"
	"strings"
)

type accountKey struct{}

func CurrentAccount(ctx context.Context) Account { a, _ := ctx.Value(accountKey{}).(Account); return a }
func bearer(r *http.Request) string {
	if len(r.Header.Values("Authorization")) != 1 {
		return ""
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		account, err := h.service.Authenticate(r.Context(), bearer(r))
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accountKey{}, account)))
	})
}
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a := CurrentAccount(r.Context())
			if !a.Active {
				writeAuthError(w, r, ErrForbidden)
				return
			}
			matched := false
			for _, role := range roles {
				if a.Role == role {
					matched = true
					break
				}
			}
			if !matched {
				writeAuthError(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
