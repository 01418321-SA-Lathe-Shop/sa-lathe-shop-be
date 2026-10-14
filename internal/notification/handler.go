package notification

import (
	"errors"
	"net/http"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("notification not found")

func Routes(r chi.Router, service *Inbox, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Get("/notifications", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			items, err := service.List(r.Context(), auth.CurrentAccount(r.Context()), limit, offset)
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": items, "limit": limit, "offset": offset})
		})
		r.Patch("/notifications/{notification_id}/read", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "notification_id")
			if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
				req.WriteError(w, r, req.ErrInvalidInput)
				return
			}
			err := service.Read(r.Context(), auth.CurrentAccount(r.Context()), id)
			if errors.Is(err, ErrNotFound) {
				err = req.ErrNotFound
			}
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			w.WriteHeader(204)
		})
	})
}
