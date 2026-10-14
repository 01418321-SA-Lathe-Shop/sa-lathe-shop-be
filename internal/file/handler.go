package file

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func actor(r *http.Request) req.Principal {
	a := auth.CurrentAccount(r.Context())
	return req.Principal{AccountID: a.ID, Role: a.Role, Active: a.Active}
}
func (s *Service) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/requests/{request_id}/uploads", func(w http.ResponseWriter, r *http.Request) {
			var in Input
			if err := req.Decode(w, r, &in); err != nil {
				req.WriteError(w, r, err)
				return
			}
			link, err := s.Start(r.Context(), actor(r), chi.URLParam(r, "request_id"), in)
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 201, link)
		})
		r.Post("/uploads/{attachment_id}/complete", func(w http.ResponseWriter, r *http.Request) {
			a, err := s.Complete(r.Context(), actor(r), chi.URLParam(r, "attachment_id"))
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, a)
		})
		r.Get("/attachments/{attachment_id}/download", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			link, err := s.Download(r.Context(), actor(r), chi.URLParam(r, "attachment_id"))
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, link)
		})
		r.Get("/requests/{request_id}/attachments", func(w http.ResponseWriter, r *http.Request) {
			items, err := s.List(r.Context(), actor(r), chi.URLParam(r, "request_id"))
			if err != nil {
				req.WriteError(w, r, err)
				return
			}
			httpserver.JSON(w, 200, map[string]any{"items": items})
		})
	})
}
