package auth

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Routes(r chi.Router) {
	r.With(NewLoginLimiter().Middleware).Post("/auth/login", h.login)
	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate)
		r.Get("/auth/me", h.me)
		r.Post("/auth/session", h.resume)
		r.Post("/auth/logout", h.logout)
		r.With(NewLoginLimiter().Middleware).Post("/auth/password", h.changePassword)
		r.Route("/admin/workers", func(r chi.Router) {
			r.Use(RequireRole(RoleAdmin))
			r.Get("/", h.listWorkers)
			r.Post("/", h.createWorker)
			r.Get("/{id}", h.getWorker)
			r.Patch("/{id}", h.updateWorker)
		})
		r.Route("/admin/admins", func(r chi.Router) {
			r.Use(RequireRole(RoleAdmin))
			r.Get("/", h.listAdmins)
			r.Post("/", h.createAdmin)
			r.Patch("/{id}", h.setAdminActive)
		})
	})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if httpserver.DecodeJSON(w, r, v, 8*1024) != nil {
		return ErrInvalidInput
	}
	return nil
}
func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "An unexpected error occurred"
	switch {
	case errors.Is(err, ErrCurrentPassword):
		status, code, message = 400, "invalid_current_password", "Current password is incorrect"
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
		status, code, message = 409, "conflict", "Account, employee number or membership already exists"
	}
	if status == http.StatusInternalServerError {
		httpserver.LogInternalError(r, err)
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
	setSessionCookie(w, result.AccessToken, result.ExpiresAt)
	httpserver.JSON(w, 200, result)
}
// resume returns the caller's current session (token and unchanged expiry) so a
// reloaded browser keeps it in memory like a fresh login; no new session is created.
func (h *Handler) resume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.service.Resume(r.Context(), sessionToken(r))
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
	token := sessionToken(r)
	if err := h.service.Logout(r.Context(), token); err != nil {
		writeAuthError(w, r, err)
		return
	}
	clearIfCurrent(w, r, token)
	w.WriteHeader(http.StatusNoContent)
}
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	if err := h.service.ChangePassword(r.Context(), CurrentAccount(r.Context()), sessionToken(r), input.CurrentPassword, input.NewPassword); err != nil {
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
	limit, offset, err := httpserver.Pagination(r)
	if err != nil {
		writeAuthError(w, r, ErrInvalidInput)
		return
	}
	workers, err := h.service.ListWorkers(r.Context(), CurrentAccount(r.Context()), limit, offset)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, map[string]any{"items": workers, "limit": limit, "offset": offset})
}
func (h *Handler) listAdmins(w http.ResponseWriter, r *http.Request) {
	admins, err := h.service.ListAdmins(r.Context(), CurrentAccount(r.Context()))
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, map[string]any{"items": admins})
}
func (h *Handler) createAdmin(w http.ResponseWriter, r *http.Request) {
	var input AdminInput
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	admin, err := h.service.CreateAdmin(r.Context(), CurrentAccount(r.Context()), input)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 201, admin)
}
func (h *Handler) setAdminActive(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Active *bool `json:"is_active"`
	}
	if err := decode(w, r, &input); err != nil {
		writeAuthError(w, r, err)
		return
	}
	admin, err := h.service.SetAdminActive(r.Context(), CurrentAccount(r.Context()), chi.URLParam(r, "id"), input.Active)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, admin)
}
