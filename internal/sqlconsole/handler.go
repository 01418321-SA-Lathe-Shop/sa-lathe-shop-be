package sqlconsole

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// current is replaceable so permission tests need no real session.
type Handler struct {
	repo    Repository
	current func(*http.Request) auth.Account
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo, func(r *http.Request) auth.Account { return auth.CurrentAccount(r.Context()) }}
}

// requireSuperAdmin: arbitrary SQL can read and destroy every table, so plain Admin is not enough.
func (h *Handler) requireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		a := h.current(r)
		if !a.Active || a.Role != auth.RoleAdmin || !a.SuperAdmin {
			httpserver.WriteError(w, r, http.StatusForbidden, "forbidden", "SQL console is limited to Super Admin")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Route("/admin/sql", func(r chi.Router) {
		r.Use(authenticate, h.requireSuperAdmin)
		r.Get("/schema", h.schema)
		r.Post("/execute", h.execute)
	})
}

func (h *Handler) schema(w http.ResponseWriter, r *http.Request) {
	tables, err := h.repo.Schema(r.Context())
	if err != nil {
		httpserver.LogInternalError(r, err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Could not read the schema")
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SQL     string `json:"sql"`
		Confirm bool   `json:"confirm"`
	}
	if httpserver.DecodeJSON(w, r, &in, MaxSQLBytes+1024) != nil || len(in.SQL) > MaxSQLBytes {
		httpserver.WriteError(w, r, http.StatusBadRequest, "invalid_request", "Request body is invalid or the statement exceeds 64 KB")
		return
	}
	info, err := Analyze(in.SQL)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, "statement_rejected", err.Error())
		return
	}
	if info.Destructive && !in.Confirm {
		httpserver.WriteError(w, r, http.StatusConflict, "confirmation_required", info.Reason)
		return
	}
	res, err := h.repo.ExecuteSQL(r.Context(), in.SQL, StatementTTL, MaxRows)
	slog.Info("sql console", "account_id", h.current(r).ID, "verb", info.Verb, "destructive", info.Destructive, "ok", err == nil, "request_id", middleware.GetReqID(r.Context()))
	var dbErr *DBError
	switch {
	case errors.As(err, &dbErr):
		httpserver.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{"code": "sql_error", "message": dbErr.Message, "sqlstate": dbErr.SQLState, "position": dbErr.Position, "detail": dbErr.Detail, "hint": dbErr.Hint, "request_id": middleware.GetReqID(r.Context())}})
	case err != nil:
		httpserver.LogInternalError(r, err)
		httpserver.WriteError(w, r, http.StatusInternalServerError, "internal_error", "Could not run the statement")
	default:
		httpserver.JSON(w, http.StatusOK, res)
	}
}
