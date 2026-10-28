package dashboard

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func queryAllowed(q url.Values, allowed ...string) bool {
	for key, values := range q {
		found := false
		for _, name := range allowed {
			found = found || key == name
		}
		if !found || len(values) != 1 || values[0] == "" {
			return false
		}
	}
	return true
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if !staff(auth.CurrentAccount(r.Context())) {
					req.WriteError(w, r, req.ErrForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/dashboard/overview", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if !queryAllowed(q, "range") {
				req.WriteError(w, r, req.ErrInvalidInput)
				return
			}
			v, e := h.service.Overview(r.Context(), auth.CurrentAccount(r.Context()), q.Get("range"))
			if e != nil {
				req.WriteError(w, r, e)
				return
			}
			httpserver.JSON(w, 200, v)
		})
		r.Get("/dashboard/jobs", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if !queryAllowed(q, "range", "status", "first_completed_on", "limit", "offset") {
				req.WriteError(w, r, req.ErrInvalidInput)
				return
			}
			limit, offset := 20, 0
			var e error
			if q.Has("limit") {
				limit, e = strconv.Atoi(q.Get("limit"))
			}
			if e == nil && q.Has("offset") {
				offset, e = strconv.Atoi(q.Get("offset"))
			}
			if e != nil {
				req.WriteError(w, r, req.ErrInvalidInput)
				return
			}
			v, e := h.service.Jobs(r.Context(), auth.CurrentAccount(r.Context()), q.Get("range"), q.Get("status"), q.Get("first_completed_on"), limit, offset)
			if e != nil {
				req.WriteError(w, r, e)
				return
			}
			httpserver.JSON(w, 200, v)
		})
	})
}
