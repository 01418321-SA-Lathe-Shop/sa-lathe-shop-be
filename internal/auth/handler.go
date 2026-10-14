package auth

import (
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"io"
	"mime"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Routes(r chi.Router) {
	r.With(NewLoginLimiter().Middleware).Post("/auth/login", h.login)
	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate)
		r.Get("/auth/me", h.me)
		r.Post("/auth/logout", h.logout)
		r.Route("/admin/workers", func(r chi.Router) {
			r.Use(RequireRole(RoleAdmin))
			r.Get("/", h.listWorkers)
			r.Post("/", h.createWorker)
			r.Get("/{id}", h.getWorker)
			r.Patch("/{id}", h.updateWorker)
		})
	})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return ErrInvalidInput
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return ErrInvalidInput
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalidInput
	}
	return nil
}
func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "An unexpected error occurred"
	switch {
	case errors.Is(err, ErrInvalidInput):
		status, code, message = 400, "invalid_input", err.Error()
	case errors.Is(err, ErrUnauthenticated):
		status, code, message = 401, "unauthenticated", "Invalid credentials or session"
		w.Header().Set("WWW-Authenticate", "Bearer")
	case errors.Is(err, ErrForbidden):
		status, code, message = 403, "forbidden", "Permission denied"
	case errors.Is(err, ErrNotFound):
		status, code, message = 404, "not_found", "Worker not found"
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "conflict", "Username or employee number already exists"
	}
	httpserver.WriteError(w, r, status, code, message)
}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	result, err := h.service.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, result)
}
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	httpserver.JSON(w, 200, CurrentAccount(r.Context()))
}
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), bearer(r)); err != nil {
		writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) createWorker(w http.ResponseWriter, r *http.Request) {
	var input WorkerInput
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	worker, err := h.service.CreateWorker(r.Context(), CurrentAccount(r.Context()), input)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 201, worker)
}
func (h *Handler) getWorker(w http.ResponseWriter, r *http.Request) {
	worker, err := h.service.GetWorker(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, worker)
}
func (h *Handler) updateWorker(w http.ResponseWriter, r *http.Request) {
	var input WorkerUpdate
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	worker, err := h.service.UpdateWorker(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "id"), input)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, worker)
}
func (h *Handler) listWorkers(w http.ResponseWriter, r *http.Request) {
	limit, offset := 50, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeAuthError(w, r, ErrInvalidInput)
			return
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			writeAuthError(w, r, ErrInvalidInput)
			return
		}
	}
	workers, err := h.service.ListWorkers(r.Context(), CurrentAccount(r.Context()), limit, offset)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, map[string]any{"items": workers, "limit": limit, "offset": offset})
}
