package material

import (
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrConflict) {
		httpserver.WriteError(w, r, 409, "conflict", "Inventory or material plan state has changed")
		return
	}
	req.WriteError(w, r, err)
}
func actor(r *http.Request) auth.Account { return auth.CurrentAccount(r.Context()) }
func respond(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpserver.JSON(w, 200, value)
}
func (h *Handler) Routes(r chi.Router, authenticate func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		r.Post("/units", func(w http.ResponseWriter, r *http.Request) {
			var in UnitInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.CreateUnit(r.Context(), actor(r), in)
			respond(w, r, v, err)
		})
		r.Get("/units", func(w http.ResponseWriter, r *http.Request) {
			v, err := h.service.Units(r.Context(), actor(r))
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Post("/materials", func(w http.ResponseWriter, r *http.Request) {
			var in MaterialInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Create(r.Context(), actor(r), in)
			respond(w, r, v, err)
		})
		r.Patch("/materials/{material_id}", func(w http.ResponseWriter, r *http.Request) {
			var in MaterialUpdate
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Update(r.Context(), actor(r), chi.URLParam(r, "material_id"), in)
			respond(w, r, v, err)
		})
		r.Get("/materials", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Search(r.Context(), actor(r), r.URL.Query().Get("search"), limit, offset)
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Post("/materials/{material_id}/stock", func(w http.ResponseWriter, r *http.Request) {
			var in StockInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Stock(r.Context(), actor(r), chi.URLParam(r, "material_id"), in)
			respond(w, r, v, err)
		})
		r.Get("/materials/{material_id}/movements", func(w http.ResponseWriter, r *http.Request) {
			limit, offset, err := req.Pagination(r)
			if err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Movements(r.Context(), actor(r), chi.URLParam(r, "material_id"), limit, offset)
			respond(w, r, map[string]any{"items": v}, err)
		})
		r.Put("/jobs/{job_id}/materials", func(w http.ResponseWriter, r *http.Request) {
			var in PlanInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Save(r.Context(), actor(r), chi.URLParam(r, "job_id"), in)
			respond(w, r, v, err)
		})
		r.Post("/jobs/{job_id}/materials/reserve", func(w http.ResponseWriter, r *http.Request) {
			var in ReserveInput
			if err := req.Decode(w, r, &in); err != nil {
				writeError(w, r, err)
				return
			}
			v, err := h.service.Reserve(r.Context(), actor(r), chi.URLParam(r, "job_id"), in)
			respond(w, r, v, err)
		})
		r.Get("/jobs/{job_id}/materials", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			v, err := h.service.Get(r.Context(), actor(r), chi.URLParam(r, "job_id"))
			respond(w, r, v, err)
		})
	})
}
