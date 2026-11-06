package production

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
	service *Service
	photos  *PhotoService
}

func NewHandler(s *Service, p *PhotoService) *Handler { return &Handler{s, p} }
func respond(w http.ResponseWriter, r *http.Request, v any, e error) {
	if errors.Is(e, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Production state or command has changed")
		return
	}
	if e != nil {
		req.WriteError(w, r, e)
		return
	}
	httpserver.JSON(w, 200, v)
}
func jsonEndpoint[T any](fn func(*http.Request, T) (any, error)) http.HandlerFunc {
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
	// Job photos are also shown to the job's customer (UC8 owner change 2026-11-06); the service and repository gate access.
	router.With(authenticate).Get("/job-photos/{photo_id}/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		v, e := h.photos.Download(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "photo_id"))
		respond(w, r, v, e)
	})
	router.Group(func(router chi.Router) {
		router.Use(authenticate)
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if e := permitted(auth.CurrentAccount(r.Context()), "staff"); e != nil {
					respond(w, r, nil, e)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		router.Get("/production/jobs", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, e := req.Pagination(r)
			if e != nil {
				respond(w, r, nil, e)
				return
			}
			v, e := h.service.List(r.Context(), auth.CurrentAccount(r.Context()), limit, offset)
			respond(w, r, map[string]any{"items": v}, e)
		})
		router.Get("/production/jobs/{job_id}", func(w http.ResponseWriter, r *http.Request) {
			v, e := h.service.Get(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"))
			respond(w, r, v, e)
		})
		router.With(auth.RequireRole(auth.RoleAdmin)).Put("/production/jobs/{job_id}/labor-cost", jsonEndpoint(func(r *http.Request, in LaborInput) (any, error) {
			return h.service.Labor(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
		}))
		router.Group(func(router chi.Router) {
			router.Use(auth.RequireRole(auth.RoleWorker))
			router.Post("/production/jobs/{job_id}/start", jsonEndpoint(func(r *http.Request, in StartInput) (any, error) {
				return h.service.Start(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
			}))
			router.Post("/production/jobs/{job_id}/usage", jsonEndpoint(func(r *http.Request, in UsageInput) (any, error) {
				return h.service.Use(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
			}))
			router.Post("/production/jobs/{job_id}/time", jsonEndpoint(func(r *http.Request, in TimeInput) (any, error) {
				return h.service.Time(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
			}))
			router.Post("/production/jobs/{job_id}/complete", jsonEndpoint(func(r *http.Request, in Command) (any, error) {
				return h.service.Complete(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
			}))
			router.Post("/production/jobs/{job_id}/photos/uploads", jsonEndpoint(func(r *http.Request, in file.Input) (any, error) {
				return h.photos.Start(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "job_id"), in)
			}))
			router.Post("/job-photos/{photo_id}/complete", func(w http.ResponseWriter, r *http.Request) {
				v, e := h.photos.Complete(r.Context(), auth.CurrentAccount(r.Context()), chi.URLParam(r, "photo_id"))
				respond(w, r, v, e)
			})
		})
	})
}
